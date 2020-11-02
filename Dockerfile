FROM golang:latest as builder
WORKDIR /app/
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=1.0.0 -X main.commitHash=$(git rev-parse HEAD) -X main.buildDate=$(date +%FT%T%z)" github.com/aplulu/bs-ranked-playlist

FROM alpine:latest
MAINTAINER Aplulu <aplulu.liv@gmail.com>
WORKDIR /app/
VOLUME ["/app/dist"]
RUN apk add --no-cache ca-certificates xz && update-ca-certificates \
  && mkdir -p /app/dist
COPY images /app/images
COPY --from=builder /app/bs-ranked-playlist /usr/bin/bs-ranked-playlist

ENTRYPOINT ["/usr/bin/bs-ranked-playlist"]

