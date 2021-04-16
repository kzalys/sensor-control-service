#!/bin/bash

INFLUXDB_TOKEN=`cat /credentials/grafana_token`

if [ $(curl --write-out '%{http_code}' --silent --output /dev/null http://$GF_SECURITY_ADMIN_USER:$GF_SECURITY_ADMIN_PASSWORD@localhost:8002/api/datasources/name/InfluxDB) -eq 404 ]; then
  echo "Creating InfluxDB datasource"
  curl --silent -XPOST -H "Content-type: application/json" -d "{
    \"orgId\": 1,
    \"name\": \"InfluxDB\",
    \"type\": \"influxdb\",
    \"access\": \"proxy\",
    \"url\": \"http://localhost:8001\",
    \"basicAuth\": false,
    \"isDefault\": true,
    \"jsonData\": {
        \"httpMode\": \"POST\",
        \"version\": \"Flux\",
        \"httpHeaderName1\": \"Authorization\",
        \"organization\": \"$INFLUXDB_ORG\",
        \"defaultBucket\": \"$INFLUXDB_BUCKET\"
    },
    \"secureJsonFields\": {},
    \"readOnly\": false,
    \"secureJsonData\": {
        \"httpHeaderValue1\": \"Token $INFLUXDB_TOKEN\",
        \"token\": \"$INFLUXDB_TOKEN\"
    }
  }" "http://$GF_SECURITY_ADMIN_USER:$GF_SECURITY_ADMIN_PASSWORD@localhost:8002/api/datasources"
  echo "Created InfluxDB datasource"
fi

if [ $(curl http://$GF_SECURITY_ADMIN_USER:$GF_SECURITY_ADMIN_PASSWORD@localhost:8002/api/alert-notifications | grep -c "\"name\":\"SCS\"") -ne 1 ]; then
  echo "Creating SCS notification channel"
  curl --silent -XPOST -H "Content-type: application/json" -d "{
   \"name\":\"SCS\",
   \"type\":\"webhook\",
   \"sendReminder\":true,
   \"disableResolveMessage\":false,
   \"frequency\":\"30s\",
   \"settings\":{
      \"uploadImage\":false,
      \"autoResolve\":true,
      \"httpMethod\":\"PUT\",
      \"severity\":\"critical\",
      \"url\":\"http://localhost:8000/configs/pushIntervals?scale=1.3\",
      \"username\":\"$GF_SECURITY_ADMIN_USER\"
   },
   \"secureSettings\":{
      \"password\":\"$GF_SECURITY_ADMIN_PASSWORD\"
   },
   \"secureFields\":{

   },
   \"isDefault\":false
}" "http://$GF_SECURITY_ADMIN_USER:$GF_SECURITY_ADMIN_PASSWORD@localhost:8002/api/alert-notifications"
  echo "Created SCS notification channel"
fi

if [ $(curl --write-out '%{http_code}' --silent --output /dev/null http://$GF_SECURITY_ADMIN_USER:$GF_SECURITY_ADMIN_PASSWORD@localhost:8002/api/dashboards/db/system-maintenance) -eq 404 ]; then
  echo "Creating system maintenance dashboard"
  DASHBOARD=`cat /home/system-maintenance-dashboard.json`
  curl -XPOST -H "Content-type: application/json" -d "${DASHBOARD//INFLUXDB_BUCKET/$INFLUXDB_BUCKET}" "http://$GF_SECURITY_ADMIN_USER:$GF_SECURITY_ADMIN_PASSWORD@localhost:8002/api/dashboards/db"
  echo "Created system maintenance dashboard"
fi
