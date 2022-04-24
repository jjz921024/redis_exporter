.PHONE: build
build:
	rm -rf apps | true && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o apps/redis_exporter && \
	echo "done"

.PHONE: clean
clean:
	rm -rf apps

.PHONE: dist
dist: build
	tar -cvf redis_exporter.tar ../redis_exporter/apps && tar -rvf redis_exporter.tar ../redis_exporter/bin
	# tar -cvf redis_exporter.tar -C apps . && tar -rvf redis_exporter.tar -C bin .

.PHONE dist-zip
dist: build
	zip redis_exporter.zip ../redis_exporter/apps/* ../redis_exporter/bin/*
	#zip -j redis_exporter.zip apps/* bin/*