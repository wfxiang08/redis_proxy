// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	"git.chunyu.me/infra/redis_proxy/pkg/proxy/redis"
	"git.chunyu.me/infra/redis_proxy/pkg/utils/atomic2"
	"git.chunyu.me/infra/redis_proxy/pkg/utils/errors"
	log "git.chunyu.me/infra/redis_proxy/pkg/utils/rolling_log"
	"git.chunyu.me/infra/redis_proxy/pkg/utils"
)

type Session struct {
	*redis.Conn

	Ops         int64

	LastOpUnix  int64
	CreateUnix  int64

	auth        string
	authorized  bool

	quit        bool
	failed      atomic2.Bool

	redisConfig *RedisConfig

	stop        sync.Once
	// 如果存在多个Master, 则第一个为主要的Master, 其他的为异步双写接口
	backendWs   []*BackendConn
	// 如果指定了Slave, 则从slave读取数据
	backendR    *BackendConn
}

func (s *Session) String() string {
	o := &struct {
		Ops        int64  `json:"ops"`
		LastOpUnix int64  `json:"lastop"`
		CreateUnix int64  `json:"create"`
		RemoteAddr string `json:"remote"`
	}{
		s.Ops, s.LastOpUnix, s.CreateUnix,
		s.Conn.Sock.RemoteAddr().String(),
	}
	b, _ := json.Marshal(o)
	return string(b)
}

func NewSession(c net.Conn, redisConfig *RedisConfig) *Session {
	return NewSessionSize(c, redisConfig, 1024 * 32, 1800)
}

func NewSessionSize(c net.Conn, redisConfig *RedisConfig, bufsize int, timeout int) *Session {
	s := &Session{CreateUnix: time.Now().Unix(), redisConfig: redisConfig}

	s.Conn = redis.NewConnSize(c, bufsize)
	s.Conn.ReaderTimeout = time.Second * time.Duration(timeout)
	s.Conn.WriterTimeout = time.Second * 30
	log.Printf("session [%p] create: %s", s, s)

	// 创建到各个backend Write的连接
	for i := 0; i < len(redisConfig.Master); i++ {
		s.backendWs = append(s.backendWs, NewBackendConn(redisConfig.Master[i]))
	}
	// 创建到Slave的连接
	if len(redisConfig.Slaves) == 1 {
		s.backendR = NewBackendConn(redisConfig.Slaves[0])
	}

	return s
}

func (s *Session) Close() error {
	err := s.Conn.Close()

	// Sleep 1s, 保证后端的事情处理完毕
	time.Sleep(time.Second * 1)

	s.stop.Do(func() {
		if s.backendWs != nil {
			for i := 0; i < len(s.backendWs); i++ {
				log.Printf(utils.Red("Close Backend: %s"), s.backendWs[i].addr)
				s.backendWs[i].Close()
			}
			s.backendWs = nil
		}
		if s.backendR != nil {
			s.backendR.Close()
			s.backendR = nil
		}
	})

	return err
}

func (s *Session) Serve(maxPipeline int) {
	var errlist errors.ErrorList
	defer func() {
		if err := errlist.First(); err != nil {
			log.Printf("session [%p] closed: %s, error = %s", s, s, err)
		} else {
			log.Printf("session [%p] closed: %s, quit", s, s)
		}
		s.Close()
	}()

	tasks := make(chan *Request, maxPipeline)
	go func() {
		defer func() {
			for _ = range tasks {
			}
		}()

		// 将请求写回client
		if err := s.loopWriter(tasks); err != nil {
			errlist.PushBack(err)
		}

		// 关闭Session
		// 关闭所有的后端连接
		s.Close()
	}()

	defer close(tasks)

	if err := s.loopReader(tasks); err != nil {
		errlist.PushBack(err)
	}
}

func (s *Session) loopReader(tasks chan <- *Request) error {

	for !s.quit {
		// 读取来自: Client的请求
		resp, err := s.Reader.Decode()
		if err != nil {
			return err
		}

		// 交给后端处理
		r, err := s.handleRequest(resp)
		if err != nil {
			return err
		} else {
			tasks <- r
		}
	}
	return nil
}

func (s *Session) loopWriter(tasks <-chan *Request) error {
	for r := range tasks {
		resp, err := s.handleResponse(r)
		if err != nil {
			return err
		}

		// 直接写出去，并且Flush
		if err := s.Writer.Encode(resp, true); err != nil {
			return err
		}
	}
	return nil
}

var ErrRespIsRequired = errors.New("resp is required")

func (s *Session) handleResponse(r *Request) (*redis.Resp, error) {
	// 读写如何同步状态呢?
	r.Wait.Wait()
	if r.Coalesce != nil {
		if err := r.Coalesce(); err != nil {
			return nil, err
		}
	}
	resp, err := r.Response.Resp, r.Response.Err
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, ErrRespIsRequired
	}
	incrOpStats(r.OpStr, microseconds() - r.Start)
	return resp, nil
}

func CloneRequest(request*Request) *Request {
	return &Request{
		OpStr:  request.OpStr,
		Start:  request.Start,
		Resp:   request.Resp,
		Wait:   nil,
		Failed: nil,
	}
}

// 如何处理Redis请求呢?
func (s *Session) handleRequest(resp *redis.Resp) (*Request, error) {
	opstr, err := getOpStr(resp)
	if err != nil {
		return nil, err
	}

	usnow := microseconds()
	s.LastOpUnix = usnow / 1e6
	s.Ops++

	r := &Request{
		OpStr:  opstr,
		Start:  usnow,
		Resp:   resp,
		Wait:   &sync.WaitGroup{},
		Failed: &s.failed,
	}

	if opstr == "QUIT" {
		return s.handleQuit(r)
	}



	// 普通的请求交给Dispatch
	// 设计:
	// 只是作为一个传话筒，不做请求合并


	// 分配请求:
	// 做请求分发
	switch opstr {
	// 如果指定SLAVE OF操作，则直接退出
	case "SLAVEOF":
		return s.handleQuit(r)

	case "AUTH":
		fallthrough
	case "PING":
		fallthrough
	case "SELECT":
		// 同时SELECT所有的服务器
		// 同时ping所有的服务器
		var r1*Request
		for i := 0; i < len(s.backendWs); i++ {
			if i != 0 {
				r1 = CloneRequest(r)
			} else {
				r1 = r
			}
			s.backendWs[i].PushBack(r1)
		}
		if s.backendR != nil {
			r1 = CloneRequest(r)
			s.backendR.PushBack(r1)
		}
	default:
		if IsReadOnlyCommand(opstr) {
			//log.Infof("Opstr: %s, is Readonly", opstr)
			if s.backendR != nil {
				// 通过只读的backendR来读取数据

				log.Debugf("Use Slave Redis: %s for: %s", s.backendR.addr, opstr)
				s.backendR.PushBack(r)
			} else {
				// 同构Ws[0]来读取数据
				log.Debugf("Use Master Redis: %s for: %s", s.backendWs[0].addr, opstr)
				s.backendWs[0].PushBack(r)
			}
		} else {
			//log.Infof("Opstr: %s, is not Readonly", opstr)
			var r1*Request
			// 多次写入数据
			for i := 0; i < len(s.backendWs); i++ {
				if i != 0 {
					r1 = CloneRequest(r)
				} else {
					r1 = r
				}

				log.Debugf("Use Master Redis: %s for: %s", s.backendWs[i].addr, opstr)
				s.backendWs[i].PushBack(r1)
			}
		}

	}

	return r, nil
}

func (s *Session) handleQuit(r *Request) (*Request, error) {
	s.quit = true
	r.Response.Resp = redis.NewString([]byte("OK"))
	return r, nil
}

func microseconds() int64 {
	return time.Now().UnixNano() / int64(time.Microsecond)
}
