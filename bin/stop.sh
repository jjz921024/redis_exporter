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
if [ -f bin/sys.pid ]; then
  pid=`cat bin/sys.pid`
  if [ -z "$pid" ];then
        echo -e "INFO\t the server does NOT start yet, there is no need to execute stop.sh."
        exit 0;
  fi

  ! kill $pid 2> /dev/null
  if [ $? -eq 0 ]; then
    echo "stop exporter"
    rm bin/sys.pid
    exit 0
  fi
fi