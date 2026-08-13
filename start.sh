#!/bin/sh
PORT=5000 ./server &
npx peer --port 5001 --path /peerjs 2>&1 | grep -v "ExperimentalWarning" &
nginx -g "daemon off;"
