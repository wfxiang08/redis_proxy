# Redis Proxy
## 功能
* 使用方式
```bash
./proxy -c redis.json
```

* 特性:
	* 可以同时绑定多个端口
	* 支持多主同时写的模式
	* 支持读写分离
		* 多个master, 一个slave(暂不支持多个slave上的读写分离)

* redis.json的格式
```json
[
  {
    "listen": ":9000",
    "master": [
      "127.0.0.1:6379",
      "127.0.0.1:6479"
    ],
    "slave": [
      "127.0.0.1:6479"
    ]
  },
  {
    "listen": ":9001",
    "master": [
      "127.0.0.1:6380",
      "127.0.0.1:6480"
    ],
    "slave": [
      "127.0.0.1:6480"
    ]
  }
]
```
* 参数说明:
	* proxy会绑定到端口 9000, 可以通过 redis-cli -p 9000 访问proxy
	* proxy在select, auth, ping时会将请求同时发送到: master0, master1, slave0
	* proxy在读数据的时候，会从slave0读取
	* proxy在写数据的时候，会同时写入 master0, master1
* 使用场景
	* 读写分离
		* 从master写入，从slave读取
		* 例如: redis的master在天津， slave在ucloud， 这样可以维持天津redis的一致性, slave的redis会稍微有一点延迟
    * 双写
	    * 在写入ucloud机器时，会同时写一份到天津机房，保证天津机房的数据的一致性（便于回滚)
