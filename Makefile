.PHONE: build
build:
	rm -rf .build | true && \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/redis_exporter && \
	echo "done"

.PHONE: clean
clean:
	rm -rf build

.PHONE: dist
dist: build
	tar -cvf redis-exporter.tar -C build . && tar -rvf redis-exporter.tar -C dist .

.PHONE dist-zip
dist: build
	zip -j redis-exporter.zip build/redis_exporter dist/start.sh dist/stop.sh