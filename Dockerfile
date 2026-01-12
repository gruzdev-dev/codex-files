FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/app
RUN CGO_ENABLED=0 GOOS=linux go build -o migrator ./cmd/migrator

FROM alpine:latest AS app

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /app/migrator .

CMD ["./main"]

