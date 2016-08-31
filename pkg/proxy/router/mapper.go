// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package router

import (
	"strings"

	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/errors"
)

var charmap [128]byte

func init() {
	for i := 0; i < len(charmap); i++ {
		c := byte(i)
		if c >= 'a' && c <= 'z' {
			c = c - 'a' + 'A'
		}
		charmap[i] = c
	}
}

var (
	readOnlyCommands = make(map[string]bool)
)

func init() {
	for _, s := range []string{
		"info", "smembers", "hlen", "hmget", "srandmember", "hvals", "randomkey", "strlen",
		"dbsize", "keys", "ttl", "lindex", "type", "llen", "dump", "scard", "echo", "lrange",
		"zcount", "exists", "sdiff", "zrange", "mget", "zrank", "get", "getbit", "getrange",
		"zrevrange", "zrevrangebyscore", "hexists", "object", "sinter", "zrevrank", "hget",
		"zscore", "hgetall", "sismember",
	} {
		readOnlyCommands[s] = true
	}
}

func IsReadOnlyCommand(opstr string) bool {
	return readOnlyCommands[opstr]
}

var (
	ErrBadRespType = errors.New("bad resp type for command")
	ErrBadOpStrLen = errors.New("bad command length, too short or too long")
)


// 获取操作Str
func getOpStr(resp *redis.Resp) (string, error) {
	if !resp.IsArray() || len(resp.Array) == 0 {
		return "", ErrBadRespType
	}
	for _, r := range resp.Array {
		if r.IsBulkBytes() {
			continue
		}
		return "", ErrBadRespType
	}

	var upper [64]byte

	var op = resp.Array[0].Value
	if len(op) == 0 || len(op) > len(upper) {
		return "", ErrBadOpStrLen
	}
	for i := 0; i < len(op); i++ {
		c := uint8(op[i])
		if k := int(c); k < len(charmap) {
			upper[i] = charmap[k]
		} else {
			return strings.ToUpper(string(op)), nil
		}
	}
	return string(upper[:len(op)]), nil
}
