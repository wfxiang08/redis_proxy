// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package utils

import (
	"os"
	"path/filepath"

	"github.com/c4pt0r/cfg"
	"git.chunyu.me/infra/redis_proxy/pkg/utils/errors"
	"git.chunyu.me/infra/redis_proxy/pkg/utils/log"
)

func InitConfig() (*cfg.Cfg, error) {
	configFile := os.Getenv("CODIS_CONF")
	if len(configFile) == 0 {
		configFile = "config.ini"
	}
	ret := cfg.NewCfg(configFile)
	if err := ret.Load(); err != nil {
		return nil, errors.Trace(err)
	} else {
		return ret, nil
	}
}

func GetExecutorPath() string {
	filedirectory := filepath.Dir(os.Args[0])
	execPath, err := filepath.Abs(filedirectory)
	if err != nil {
		log.PanicErrorf(err, "get executor path failed")
	}
	return execPath
}

type Strings []string

func (s1 Strings) Eq(s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}
