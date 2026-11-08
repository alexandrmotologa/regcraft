# Stage 1: Build Web UI
FROM node:24-alpine AS ui-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Binary
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /web/dist ./internal/ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=1.0.0" -o /bin/regcraft ./cmd/regcraft

# Stage 3: Minimal Scratch Image
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /data
VOLUME ["/data"]
COPY --from=builder /bin/regcraft /usr/local/bin/regcraft

EXPOSE 5000
ENTRYPOINT ["/usr/local/bin/regcraft"]
CMD ["serve", "--dir", "/data", "--port", "5000"]
