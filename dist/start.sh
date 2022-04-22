#!/bin/sh

nohup ./redis_exporter --web.listen-address=:9121 -cluster-name=xxxx -export-client-list > exporter.log 2>&1 & echo $! > sys.pid

exit 0