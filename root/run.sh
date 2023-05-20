#!/usr/bin/env bash

_kill() {
    echo "[WARN] Receive SIGTERM, kill $pids"
    for pid in $pids; do
        kill "$pid"
        wait "$pid"
    done
    exit 0
}

## 识别中断信号，停止进程
trap _kill HUP INT PIPE QUIT TERM

/bak.sh &

while true; do sleep 5; done
