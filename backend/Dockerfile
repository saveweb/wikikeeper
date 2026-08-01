# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o extension-migrator ./cmd/extension-migrator

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /app/extension-migrator .
COPY --from=builder /app/web ./web

EXPOSE 8000

CMD ["./server"]
