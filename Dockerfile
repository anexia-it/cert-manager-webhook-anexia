FROM golang:1.27-alpine AS build_deps

LABEL org.opencontainers.image.source="https://github.com/anexia-it/cert-manager-webhook-anexia"

RUN apk add --no-cache git

WORKDIR /workspace

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

ARG VERSION=dev

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -o webhook -ldflags "-s -w -X main.Version=${VERSION}" .

FROM alpine:3.24

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 1001 app \
    && adduser -S -u 1001 -G app app

COPY --from=build /workspace/webhook /usr/local/bin/webhook

USER 1001:1001

ENTRYPOINT ["webhook"]
