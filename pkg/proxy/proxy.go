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
	redisConfig *RedisConfig
	listener    net.Listener

	kill        chan interface{}
	wait        sync.WaitGroup
	stop        sync.Once
}

func New(redisConfig *RedisConfig) *Server {

	s := &Server{redisConfig:redisConfig}
	s.kill = make(chan interface{})

	// 监听某个端口
	if l, err := net.Listen("tcp", redisConfig.Listen); err != nil {
		log.PanicErrorf(err, "open listener failed")
	} else {
		s.listener = l
	}

	// 添加一个Wait
	s.wait.Add(1)

	// 异步执行
	go func() {
		// server结束: wait释放
		// 很难做到: gracefully stop, 要关闭了就直接关闭吧
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


	maxPipeline := 10

	// 为每个请求建一个Session
	go func() {
		for c := range ch {
			x := router.NewSessionSize(c, s.redisConfig, 1024 * 1024, 5000)

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
	// 等待: wait释放
	s.wait.Wait()
}

func (s *Server) Close() error {
	// close 之后，等待: socket的结束
	s.close()

	// 等待Wait释放
	s.wait.Wait()
	return nil
}

func (s *Server) close() {
	// 一次性的动作
	s.stop.Do(func() {
		s.listener.Close()
		close(s.kill)
	})
}

