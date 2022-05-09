#!/bin/sh

PRG="$0"
while [ -h "$PRG" ] ; do
    ls=`ls -ld "$PRG"`
    link=`expr "$ls" : '.*-> \(.*\)$'`
    if expr "$link" : '/.*' > /dev/null; then
        PRG="$link"
    else
        PRG=`dirname "$PRG"`"/$link"
    fi
done
SAVED="`pwd`"
cd "`dirname \"$PRG\"`/.." >/dev/null
APP_HOME="`pwd -P`"

cd ${APP_HOME}

ARGS="--web.listen-address=:9121 -export-client-list"

if [ ! -f conf/rds.txt ]; then
    echo "not found rds.txt file"
    exit -1
fi

PWD=`cat conf/rds.txt`
if [ -n "$PWD" ]; then
    ARGS="$ARGS -redis.password=$PWD"
fi

nohup ./apps/redis-exporter ${ARGS} > exporter.log 2>&1 & echo $! > bin/sys.pid

exit 0