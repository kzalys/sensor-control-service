include .env
export $(shell sed 's/=.*//' .env)

default: up

up:
	docker-compose up

down:
	docker-compose down

cleanup: down
	docker volume rm sensor-control-service_credentials & true
