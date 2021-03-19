#!/bin/sh
set -e

if [ "$(influx bucket list -n internal_metrics | grep -c 'not found')" -eq 0 ]; then exit 0; fi

ADMIN_TOKEN=$(influx auth list -u "$ADMIN_USERNAME" --json | jq '.[0].token' --raw-output)
ORG_ID=$(influx org list -n "$INFLUXDB_ORG" --json --token "$ADMIN_TOKEN" | jq '.[0].id' --raw-output)
INTERNAL_BUCKET_ID=$(influx bucket create -o "$INFLUXDB_ORG" -n internal_metrics --json --token "$ADMIN_TOKEN" | jq '.id' --raw-output)

curl --location --request POST 'http://localhost:8086/api/v2/scrapers' \
--header "Authorization: Token $ADMIN_TOKEN" \
--header 'Content-Type: application/json' \
--data-raw "{
  \"name\": \"internal metrics\",
  \"type\": \"prometheus\",
  \"url\": \"http://localhost:8086/metrics\",
  \"orgID\": \"$ORG_ID\",
  \"bucketID\": \"$INTERNAL_BUCKET_ID\",
  \"allowInsecure\": false
}"
