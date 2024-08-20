FROM golang:1.23.0-alpine AS builder
WORKDIR /app-build
COPY . .
RUN go mod tidy
RUN go build -o server ./server.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /app-build/server .
EXPOSE 8080
ENTRYPOINT ["./server"]