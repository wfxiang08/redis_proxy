./build.sh
mkdir -p /usr/local/redis_proxy/bin
mkdir -p /usr/local/redis_proxy/log

rm -rf /usr/local/redis_proxy/bin/redis_proxy
cp redis_proxy /usr/local/redis_proxy/bin/redis_proxy
cp scripts/redis_proxy.service /lib/systemd/system/
systemctl daemon-reload
systemctl restart redis_proxy

tail -f /usr/local/redis_proxy/log/redis_proxy.log-`date +"%Y%m%d"`