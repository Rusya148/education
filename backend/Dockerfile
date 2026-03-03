FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o minion-bank-backend .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/minion-bank-backend .

EXPOSE 8080
CMD ["./minion-bank-backend"]
