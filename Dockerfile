FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /glide .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S glide \
    && adduser -S -G glide -u 65532 glide \
    && mkdir -p /app/sessions \
    && chown -R glide:glide /app

WORKDIR /app
USER glide

COPY --from=builder /glide /usr/local/bin/glide

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/glide"]
CMD ["api"]
