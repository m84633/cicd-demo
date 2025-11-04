#!/bin/bash
set -euo pipefail

for host in mongo rabbitmq redis; do
  case "$host" in
    mongo) port=27017 ;;
    rabbitmq) port=5672 ;;
    redis) port=6379 ;;
  esac
  echo "Waiting for $host:$port..."
  while ! timeout 1 bash -c "</dev/tcp/$host/$port" >/dev/null 2>&1; do
    sleep 1
  done
  echo "$host:$port is reachable"

done

exec /app/server serve:console -c /app/config/e2e/config.yaml
