FROM golang:1.23-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/shipr-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/shipr-worker ./cmd/worker

FROM alpine:3.20

RUN apk add --no-cache ca-certificates git docker-cli curl

WORKDIR /app
COPY --from=builder /app/shipr-api /app/shipr-api
COPY --from=builder /app/shipr-worker /app/shipr-worker
COPY migrations /app/migrations

EXPOSE 8080
CMD ["/app/shipr-api"]
