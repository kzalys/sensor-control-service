FROM influxdb:2.0-alpine

RUN mkdir /credentials
RUN chown influxdb /credentials
RUN chgrp influxdb /credentials
RUN chmod 755 /credentials
RUN adduser scs -D

COPY ./scripts/influxdb_create_internal_metrics_scrapper.sh /home/influxdb_create_internal_metrics_scrapper.sh

RUN apk add jq curl