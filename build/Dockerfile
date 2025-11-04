# 🧱 Stage 1: Build
FROM golang:1.25.3-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o app ./

# 🐋 Stage 2: Run
FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/app .

# # Non-root user (good practice)
# RUN adduser -D appuser
# USER appuser

EXPOSE 8080
CMD ["./app"]