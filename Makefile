.PHONE: build
build:
	rm -rf apps | true && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o apps/redis-exporter && \
	echo "done"

.PHONE: clean
clean:
	! rm redis-exporter.tar
	! rm redis-exporter.zip
	! rm -rf apps

.PHONE: dist
dist: build
	tar -cvf redis-exporter.tar --transform 's,^,redis-exporter/,S' apps && tar -rvf redis-exporter.tar --transform 's,^,redis-exporter/,S' bin
	# tar -cvf redis_exporter.tar -C apps . && tar -rvf redis_exporter.tar -C bin .

.PHONE: dist-zip
dist-zip: build
	zip redis-exporter.zip -u apps/* bin/*
	# zip -j redis-exporter.zip apps/* bin/*