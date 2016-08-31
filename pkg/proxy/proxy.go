// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"net"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/proxy/router"
	"github.com/CodisLabs/codis/pkg/utils/log"
)

type Server struct {
	listener net.Listener

	kill     chan interface{}
	wait     sync.WaitGroup
	stop     sync.Once
}

func New(addr string, passwd, debugVarAddr string) *Server {

	s := &Server{}
	s.kill = make(chan interface{})

	if l, err := net.Listen("tcp", addr); err != nil {
		log.PanicErrorf(err, "open listener failed")
	} else {
		s.listener = l
	}

	s.wait.Add(1)
	// 异步执行
	go func() {
		defer s.wait.Done()
		s.serve()
	}()
	return s
}

func (s *Server) serve() {
	defer s.close()

	log.Info("proxy is serving")
	s.handleConns()
}


//
// 如何处理请求
//
func (s *Server) handleConns() {
	ch := make(chan net.Conn, 4096)
	defer close(ch)

	passwd := ""
	addr := "127.0.0.1:6379"

	maxPipeline := 10
	// 为每个请求建一个Session
	go func() {
		for c := range ch {
			x := router.NewSessionSize(c, passwd, addr, 1024 * 1024, 5000)

			// Session处理Redis的请求
			go x.Serve(maxPipeline)
		}
	}()

	// 接受client的请求
	for {
		c, err := s.listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.WarnErrorf(err, "[%p] proxy accept new connection failed, get temporary error", s)
				time.Sleep(time.Millisecond * 10)
				continue
			}
			log.WarnErrorf(err, "[%p] proxy accept new connection failed, get non-temporary error, must shutdown", s)
			return
		} else {
			ch <- c
		}
	}
}

func (s *Server) Join() {
	s.wait.Wait()
}

func (s *Server) Close() error {
	s.close()
	s.wait.Wait()
	return nil
}

func (s *Server) close() {
	s.stop.Do(func() {
		s.listener.Close()
		close(s.kill)
	})
}

