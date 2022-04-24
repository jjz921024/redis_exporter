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

echo "APP_HOME: " $APP_HOME
cd ${APP_HOME}

nohup ./apps/redis-exporter --web.listen-address=:9121 -export-client-list > exporter.log 2>&1 & echo $! > bin/sys.pid

exit 0