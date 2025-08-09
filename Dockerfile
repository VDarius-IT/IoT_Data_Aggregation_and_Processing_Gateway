FROM golang:1.21-alpine AS build
WORKDIR /src
COPY . .
RUN go build -o /bin/edge-gateway ./cmd/gateway

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/edge-gateway /usr/local/bin/edge-gateway
COPY config/config.yaml /etc/edge-gateway/config.yaml
EXPOSE 9000
CMD ["/usr/local/bin/edge-gateway", "--config", "/etc/edge-gateway/config.yaml"]
