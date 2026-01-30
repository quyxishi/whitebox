FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . ./
RUN go build -o whitebox ./cmd/api

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=builder /app/whitebox /app/whitebox

ENV GIN_MODE=release

EXPOSE 9116
ENTRYPOINT ["/app/whitebox"]