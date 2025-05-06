# ---------- builder ---------
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/relay main.go

# ---------- final ---------
FROM alpine:3.20
COPY --from=builder /bin/relay /usr/local/bin/relay
RUN --mount=type=cache,target=/var/cache/apk apk add iproute2-tc netcat-openbsd
# Explicitly use IPv4 because
# $ docker exec <container> getent hosts localhost
# ::1               localhost  localhost
HEALTHCHECK --interval=10s --retries=3 CMD sh -c "nc -z 127.0.0.1 4003"

