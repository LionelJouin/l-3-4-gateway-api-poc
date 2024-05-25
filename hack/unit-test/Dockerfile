FROM golang:1.22 as build

ENV GO111MODULE=on

RUN apt-get update -y \
    && apt-get install -y iproute2 iputils-ping net-tools nftables libnetfilter-queue-dev ipvsadm

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download
