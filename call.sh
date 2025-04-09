PROMETHEUS_URL="http://localhost:9080"
START_TIME="2025-04-08T00:00:00Z"
END_TIME="2025-04-08T23:59:59Z"
STEP="3600s"
APP_NAME="ice-validator-v1"

# This query uses a 1-hour time window [1h] to calculate the increase in requests
# For longer-term trends with more smoothing, consider using [1d] instead
# See README.md section "Understanding Time Windows in Prometheus Queries" for details
QUERY="sum(increase(istio_requests_total{app=\"${APP_NAME}\"}[1h])) by (app)"

curl -g "${PROMETHEUS_URL}/api/v1/query_range" \
  --data-urlencode "query=$QUERY" \
  --data-urlencode "start=$START_TIME" \
  --data-urlencode "end=$END_TIME" \
  --data-urlencode "step=$STEP"
