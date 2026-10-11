FROM golang:alpine AS builder
WORKDIR /envelope-backend
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/envelope-api ./cmd/api

FROM alpine:3.21
RUN apk --no-cache add ca-certificates && adduser -D -H -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/envelope-api /app/envelope-api
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/envelope-api"]
