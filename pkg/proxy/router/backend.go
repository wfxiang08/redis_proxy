// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"sync"
	"time"

	"git.chunyu.me/infra/redis_proxy/pkg/proxy/redis"
	"git.chunyu.me/infra/redis_proxy/pkg/utils/errors"
	log "git.chunyu.me/infra/redis_proxy/pkg/utils/rolling_log"
)

type BackendConn struct {
	addr  string
	stop  sync.Once
	input chan *Request
}

func NewBackendConn(address string) *BackendConn {
	bc := &BackendConn{
		addr: address,
		input: make(chan *Request, 1024),
	}
	go bc.Run()
	return bc
}

func (bc *BackendConn) Run() {
	log.Infof("backend conn [%p] to %s, start service", bc, bc.addr)
	for k := 0;; k++ {
		err := bc.loopWriter()
		if err == nil {
			break
		} else {
			for i := len(bc.input); i != 0; i-- {
				r := <-bc.input
				bc.setResponse(r, nil, err)
			}
		}
		log.WarnErrorf(err, "backend conn [%p] to %s, restart [%d]", bc, bc.addr, k)
		time.Sleep(time.Millisecond * 50)
	}
	log.Infof("backend conn [%p] to %s, stop and exit", bc, bc.addr)
}

func (bc *BackendConn) Addr() string {
	return bc.addr
}

func (bc *BackendConn) Close() {
	bc.stop.Do(func() {
		close(bc.input)
	})
}

func (bc *BackendConn) PushBack(r *Request) {
	if r.Wait != nil {
		r.Wait.Add(1)
	}
	bc.input <- r
}

func (bc *BackendConn) KeepAlive() bool {
	if len(bc.input) != 0 {
		return false
	}
	r := &Request{
		Resp: redis.NewArray([]*redis.Resp{
			redis.NewBulkBytes([]byte("PING")),
		}),
	}

	select {
	case bc.input <- r:
		return true
	default:
		return false
	}
}

var ErrFailedRequest = errors.New("discard failed request")

func (bc *BackendConn) loopWriter() error {
	r, ok := <-bc.input
	if ok {
		c, tasks, err := bc.newBackendReader()
		if err != nil {
			return bc.setResponse(r, nil, err)
		}
		defer close(tasks)

		for ok {
			if bc.canForward(r) {
				// 每次都直接Flush
				if err := c.Writer.Encode(r.Resp, true); err != nil {
					return bc.setResponse(r, nil, err)
				}

				tasks <- r
			} else {
				bc.setResponse(r, nil, ErrFailedRequest)
			}

			r, ok = <-bc.input
		}
	}
	return nil
}

// 创建一个到Backend的连接
func (bc *BackendConn) newBackendReader() (*redis.Conn, chan <- *Request, error) {
	c, err := redis.DialTimeout(bc.addr, 1024 * 512, time.Second)
	if err != nil {
		return nil, nil, err
	}
	c.ReaderTimeout = time.Minute
	c.WriterTimeout = time.Minute

	tasks := make(chan *Request, 4096)
	go func() {
		defer c.Close()
		for r := range tasks {
			// 读取处理完毕的tasks
			resp, err := c.Reader.Decode()
			bc.setResponse(r, resp, err)
			if err != nil {
				// close tcp to tell writer we are failed and should quit
				c.Close()
			}
		}
	}()
	return c, tasks, nil
}

func (bc *BackendConn) canForward(r *Request) bool {
	if r.Failed != nil && r.Failed.Get() {
		return false
	} else {
		return true
	}
}

func (bc *BackendConn) setResponse(r *Request, resp *redis.Resp, err error) error {
	r.Response.Resp, r.Response.Err = resp, err
	if err != nil && r.Failed != nil {
		r.Failed.Set(true)
	}
	if r.Wait != nil {
		r.Wait.Done()
	}
	if r.slot != nil {
		r.slot.Done()
	}
	return err
}