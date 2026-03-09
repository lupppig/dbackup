# 1. Build the React UI
FROM node:20-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package.json ui/package-lock.json* ./
RUN npm install
COPY ui/ .
RUN npm run build

# 2. Build the Hugo Documentation
FROM klakegg/hugo:0.111.3-ext-alpine AS docs-builder
WORKDIR /app/docs-site
COPY docs-site/ .
RUN hugo --minify

# 3. Build the Go Binary
FROM golang:1.22-alpine AS go-builder
WORKDIR /app

# Install standard build dependencies (in case `make` or cgo is needed)
RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy the static assets into the web/ directory so Go can correctly embed them
RUN mkdir -p web/ui web/docs
COPY --from=ui-builder /app/ui/dist ./web/ui
COPY --from=docs-builder /app/docs-site/public ./web/docs

RUN go build -o dbackup main.go

# 4. Final Minimal Runtime Image
FROM alpine:latest
WORKDIR /root/

# Install CA certificates for external API/Storage calls
RUN apk --no-cache add ca-certificates tzdata

COPY --from=go-builder /app/dbackup .

# Expose the correct port for PaaS providers (Pxxl often uses the PORT env var, we map 8080 by default)
ENV PORT=8080
EXPOSE 8080

# Run the UI server
ENTRYPOINT ["./dbackup", "ui"]
