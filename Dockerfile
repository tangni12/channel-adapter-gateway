FROM node:24-alpine AS web-builder

WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci
COPY web .
RUN npm run build

FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/channel-adapter-gateway ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/channel-adapter-gateway /app/channel-adapter-gateway
COPY configs /app/configs
COPY --from=web-builder /web/dist /app/web/dist

EXPOSE 8088
ENV TZ=Asia/Shanghai
ENV GATEWAY_CONFIG=/app/configs/config.yaml
CMD ["/app/channel-adapter-gateway"]
