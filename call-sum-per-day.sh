#!/bin/bash

PROMETHEUS_URL="http://localhost:9080"
START_TIME="2025-04-08T00:00:00Z"
END_TIME="2025-04-08T23:59:59Z"
APP_NAME="memento"

# Query to get the total number of requests for the day
QUERY="sum(increase(istio_requests_total{app=\"${APP_NAME}\"}[24h])) by (app)"

# To get the value at the end of the day
END_TIME_UNIX=$(date -d "$END_TIME" +%s)

curl -g "${PROMETHEUS_URL}/api/v1/query" \
  --data-urlencode "query=$QUERY" \
  --data-urlencode "time=$END_TIME_UNIX"

