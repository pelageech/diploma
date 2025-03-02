# run-go:latest
FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod go.sum ./
COPY third-party/* ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o run-go ./cmd/hc/...
