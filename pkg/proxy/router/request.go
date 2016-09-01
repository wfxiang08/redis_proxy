// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"sync"

	"git.chunyu.me/infra/redis_proxy/pkg/proxy/redis"
	"git.chunyu.me/infra/redis_proxy/pkg/utils/atomic2"
)

type Request struct {
	OpStr    string
	Start    int64

	Resp     *redis.Resp

	Coalesce func() error
	Response struct {
				 Resp *redis.Resp
				 Err  error
			 }

	Wait     *sync.WaitGroup
	Failed   *atomic2.Bool
}
