FROM golang:1.25.3 AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . ./
RUN go build -o main ./cmd/api/main.go

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=builder /app/main /app/main

ENV GIN_MODE=release

EXPOSE 9116
ENTRYPOINT ["/app/main"]