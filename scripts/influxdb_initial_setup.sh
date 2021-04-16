#!/bin/sh
set -e

SCS_TOKEN=$(influx auth create -o "$INFLUXDB_ORG" -u "$ADMIN_USERNAME" --write-buckets --read-buckets --json -d scs \
      | jq '.token' --raw-output)
echo "$SCS_TOKEN" > /credentials/scs_token
chmod 644 /credentials/scs_token


GRAFANA_TOKEN=$(influx auth create -o "$INFLUXDB_ORG" -u "$ADMIN_USERNAME" --read-buckets --json -d grafana \
      | jq '.token' --raw-output)
echo "$SCS_TOKEN" > /credentials/grafana_token
chmod 644 /credentials/grafana_token

