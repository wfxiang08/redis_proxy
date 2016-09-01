// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.
package router


//[
// {"listen": 6378,
//  "master":{"host": "host01", "port": 6378},
//  "slave": [{"host": "host02", "port": 6380}]
// },
// {"listen": 6377,
//  "master":{"host": "host01", "port": 6379},
//  "slave": [{"host": "host02", "port": 6380}, {"host": "host03", "port": 6380}]
// }
//]


type RedisConfig struct {
	Listen string `json:"listen"`
	Master []string `json:"master"`
	Slaves []string `json:"slave"`
}