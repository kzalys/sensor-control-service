include .env
export $(shell sed 's/=.*//' .env)

default: up

up:
	docker-compose up

down:
	docker-compose down

cleanup: down
	docker volume rm sensor-control-service_credentials & true

build:
	docker build --tag grafana/grafana:scs -f ./docker/grafana/Dockerfile .
	docker build --tag influxdb:scs -f ./docker/influxdb/Dockerfile .
	docker build --tag sensor-control-service:latest -f docker/scs/Dockerfile .
