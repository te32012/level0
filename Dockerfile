# syntax=docker/dockerfile:1

FROM golang:latest
RUN mkdir -p /go/src/app

WORKDIR /go/src/app
COPY . /go/src/app
RUN go mod download

EXPOSE 8080
ENTRYPOINT go run main.go