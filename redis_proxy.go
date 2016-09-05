// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/docopt/docopt-go"
	"git.chunyu.me/infra/redis_proxy/pkg/proxy"
	"git.chunyu.me/infra/redis_proxy/pkg/utils"
	log "git.chunyu.me/infra/redis_proxy/pkg/utils/rolling_log"
	"encoding/json"
	"io/ioutil"
	"git.chunyu.me/infra/redis_proxy/pkg/proxy/router"
)

var (
	cpus = 2
)

// http://127.0.0.1:8080/debug/pprof/
var usage = `usage: proxy [-c <config_file>] [-L <log_file>] [--log-keep-days=<maxdays>]  [--log-level=<loglevel>] [--log-filesize=<filesize>] [--profile-addr=<profile-addr>]

options:
   -c	set config file
   -L	set output log file, default is stdout
   --log-level=<loglevel>	set log level: info, warn, error, debug [default: info]
   --log-keep-days=<maxdays>  set max log file keep days, default is 3 days
   --profile-addr=<profile_http_server_addr>		profile http server
`

func init() {
	log.SetLevel(log.LEVEL_INFO)
}

func setLogLevel(level string) {
	level = strings.ToLower(level)
	var l = log.LEVEL_INFO
	switch level {
	case "error":
		l = log.LEVEL_ERROR
	case "warn", "warning":
		l = log.LEVEL_WARN
	case "debug":
		l = log.LEVEL_DEBUG
	case "info":
		fallthrough
	default:
		level = "info"
		l = log.LEVEL_INFO
	}
	log.SetLevel(l)
	log.Infof("set log level to <%s>", level)
}

func setCrashLog(file string) {
	f, err := os.OpenFile(file, os.O_WRONLY | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.InfoErrorf(err, "cannot open crash log file: %s", file)
	} else {
		syscall.Dup2(int(f.Fd()), 2)
	}
}

func handleSetLogLevel(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	setLogLevel(r.Form.Get("level"))
}

func checkUlimit(min int) {
	ulimitN, err := exec.Command("/bin/sh", "-c", "ulimit -n").Output()
	if err != nil {
		log.WarnErrorf(err, "get ulimit failed")
	}

	n, err := strconv.Atoi(strings.TrimSpace(string(ulimitN)))
	if err != nil || n < min {
		log.Panicf("ulimit too small: %d, should be at least %d", n, min)
	}
}

func main() {
	args, err := docopt.Parse(usage, nil, true, utils.Version, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var redisConfigs[]router.RedisConfig
	// set config file
	if args["-c"] != nil {
		configFile := args["-c"].(string)
		f, err := os.Open(configFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// 读取配置文件
		data, _ := ioutil.ReadAll(f)
		err = json.Unmarshal(data, &redisConfigs)
		if err != nil {
			fmt.Printf("Error:%v\n", err)
			os.Exit(1)
		}

		// 验证配置文件OK
		if len(redisConfigs) == 0 {
			fmt.Printf("Error: Invalid Configuration\n")
			os.Exit(1)
		}
		for i := 0; i < len(redisConfigs); i++ {
			if len(redisConfigs[i].Master) == 0 {
				fmt.Printf("Error: Invalid Configuration\n")
				os.Exit(1)
			}
			if len(redisConfigs[i].Slaves) > 1 {
				fmt.Printf("Error: Invalid Configuration\n")
				os.Exit(1)
			}
		}

	} else {
		fmt.Printf("Error: Invalid Configuration\n")
		os.Exit(1)
	}

	var maxKeepDays int = 3
	if s, ok := args["--log-keep-days"].(string); ok && s != "" {
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			log.PanicErrorf(err, "invalid max log file keep days = %s", s)
		}
		maxKeepDays = int(v)
	}

	// set output log file
	if s, ok := args["-L"].(string); ok && s != "" {
		f, err := log.NewRollingFile(s, maxKeepDays)
		if err != nil {
			log.PanicErrorf(err, "open rolling log file failed: %s", s)
		} else {
			defer f.Close()
			log.StdLog = log.New(f, "")
		}
	}

	log.SetLevel(log.LEVEL_INFO)
	log.SetFlags(log.Flags() | log.Lshortfile)

	// set log level
	if s, ok := args["--log-level"].(string); ok && s != "" {
		setLogLevel(s)
	}

	cpus = runtime.NumCPU()
	checkUlimit(10000)
	runtime.GOMAXPROCS(cpus)


	// 这就是为什么 Codis 傻乎乎起一个 http server的目的
	if s, ok := args["--profile-addr"].(string); ok && len(s) > 0 {

		go func() {
			log.Printf(utils.Red("Profile Address: %s"), s)

			// 通过URL调整Debug模式
			// http://123.59.153.235:7171/setloglevel?level=debug
			// http://123.59.153.235:7171/setloglevel?level=info
			http.HandleFunc("/setloglevel", handleSetLogLevel)
			log.Println(http.ListenAndServe(s, nil))
		}()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, os.Kill)

	var servers[]*proxy.Server
	for i := 0; i < len(redisConfigs); i++ {
		s := proxy.New(&redisConfigs[i])
		servers = append(servers, s)
	}

	// 强制关闭所有的Server
	go func() {
		<-c
		log.Info("ctrl-c or SIGTERM found, bye bye...")
		for i := 0; i < len(servers); i++ {
			servers[i].Close()
		}
	}()

	for i := 0; i < len(servers); i++ {
		servers[i].Join()
	}

	log.Infof("proxy exit!! :(")
}