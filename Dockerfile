# syntax=docker/dockerfile:1

FROM golang:1.24-alpine

WORKDIR /app

COPY ./src ./

RUN go mod download

RUN go build -o /go-wallet-service

# HTTP Port
EXPOSE 5025

# GRPC port
EXPOSE 5000

CMD ["/go-wallet-service"]