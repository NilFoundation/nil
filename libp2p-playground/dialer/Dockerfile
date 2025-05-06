# ---------- builder ---------
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/dialer main.go

# ---------- final ---------
FROM alpine:3.20
COPY --from=builder /bin/dialer /usr/local/bin/dialer
RUN --mount=type=cache,target=/var/cache/apk apk add bind-tools jq curl tcpdump iproute2-tc

