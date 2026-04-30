# -- Builder stage --
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /src

# Cache dependency downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /token-tumbler .

# -- Final stage --
FROM gcr.io/distroless/static-debian12:nonroot

# The application reads config.yaml from its working directory at runtime.
# Mount your config.yaml into the container, e.g.:
#   docker run -v $(pwd)/config.yaml:/config.yaml ghcr.io/nabsku/token-tumbler
COPY --from=builder /token-tumbler /

WORKDIR /
ENTRYPOINT ["/token-tumbler"]