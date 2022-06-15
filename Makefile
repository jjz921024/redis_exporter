VER=v1.3.1

.PHONE: build
build:
	rm -rf apps | true && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o apps/redis-exporter && \
	echo "done"

.PHONE: clean
clean:
	rm redis-exporter-$(VER).tar.gz | true && \
	rm -rf apps | true

.PHONE: dist
dist: build
	tar -zcvf redis-exporter-$(VER).tar.gz apps/* bin/* conf/*
# tar -zcf target/redis-exporter.tar.gz redis-exporter.tar --remove-files
# tar -rvf redis-exporter.tar conf/ # 追加
# tar -cvf redis-exporter.tar --transform 's,^,redis-exporter/,S' apps # 替换目录
# tar -cvf redis_exporter.tar -C apps . && tar -rvf redis_exporter.tar -C bin . # 不保留原目录
# tar -cvf redis_exporter.tar apps/  # 保留原目录

.PHONE: dist-zip
dist-zip: build
	zip redis-exporter-$(VER).zip -u apps/* bin/*
# zip -j redis-exporter.zip apps/* bin/*