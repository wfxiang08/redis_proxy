// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"sync"

	"proxy/redis"
	"github.com/wfxiang08/cyutils/utils/atomic2"
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
