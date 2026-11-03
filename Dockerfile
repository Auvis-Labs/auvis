FROM golang:1.25-alpine AS builder

WORKDIR /build

# Cache dependencies separately from source for faster rebuilds
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=demo -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /build/auvis \
    ./cmd/auvis

FROM alpine:3.20

RUN apk update && apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/auvis /app/auvis

# Policies get mounted at runtime via volume, not baked into the image —
# so updating a policy doesn't require a rebuild
RUN mkdir -p /app/policies /data

EXPOSE 8080

ENTRYPOINT ["/app/auvis"]
CMD ["dev"]