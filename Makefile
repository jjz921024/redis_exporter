.PHONE: build
build:
	rm -rf apps | true && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o apps/redis-exporter && \
	echo "done"

.PHONE: clean
clean:
	rm redis-exporter.tar | true && \
	rm redis-exporter.zip | true && \
	rm -rf apps | true

.PHONE: dist
dist: build
	tar -cvf redis-exporter.tar --transform 's,^,redis-exporter/,S' apps && \
	tar -rvf redis-exporter.tar --transform 's,^,redis-exporter/,S' bin && \
	tar -rvf redis-exporter.tar --transform 's,^,redis-exporter/,S' conf
	# tar -cvf redis_exporter.tar -C apps . && tar -rvf redis_exporter.tar -C bin .

.PHONE: dist-zip
dist-zip: build
	zip redis-exporter.zip -u apps/* bin/*
	# zip -j redis-exporter.zip apps/* bin/*