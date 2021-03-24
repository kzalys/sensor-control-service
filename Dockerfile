FROM golang:alpine3.13

WORKDIR /go/src/github.com/kzalys/sensor-control-service

ENV GO111MODULE on

COPY . .

RUN apk add git
RUN go mod download
RUN apk del git

ENV GOPATH /go

EXPOSE 8000

ENTRYPOINT go run main.go
