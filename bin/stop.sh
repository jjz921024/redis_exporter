#!/bin/sh

cat sys.pid | xargs kill

if [ $? -eq 0 ]; then
  echo "stop exporter"
  rm sys.pid
  exit 0
fi

exit -1
