#!/usr/bin/env bash
GOOS=linux GOARCH=amd64 go build cmds/redis_proxy.go