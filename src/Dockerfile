# syntax=docker/dockerfile:1

FROM golang:1.18-alpine

WORKDIR /app

COPY . ./

RUN go mod download

RUN go build -o /go-wallet-service

# HTTP Port
EXPOSE 8080

# GRPC port
EXPOSE 8081

CMD ["/go-wallet-service"]