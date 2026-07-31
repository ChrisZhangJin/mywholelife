# syntax=docker/dockerfile:1

# --- builder: pure-Go static binary, no cgo ---
FROM golang:1.25 AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct GOFLAGS=-mod=mod

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY server ./server
COPY store ./store
COPY dream ./dream
COPY bundle ./bundle

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/mywholelife ./cmd/mywholelife

# Pre-create the data dir so the nonroot runtime can open the SQLite DB even
# without a mounted volume (distroless has no shell to mkdir at runtime).
RUN mkdir -p /out/data

# --- runtime: distroless static (ships CA certs for the dream HTTPS LLM call) ---
FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.version=1.2.0

COPY --from=builder /out/mywholelife /mywholelife
COPY --from=builder --chown=65532:65532 /out/data /data

EXPOSE 8080
ENTRYPOINT ["/mywholelife"]
CMD ["serve"]
