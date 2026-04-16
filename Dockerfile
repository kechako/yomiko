# syntax=docker/dockerfile:1
# check=skip=SecretsUsedInArgOrEnv

ARG GO_VERSION=1.26.3

#========================================
# Build
FROM golang:${GO_VERSION}-trixie AS builder

ENV DEBIAN_FRONTEND=noninteractive

# install packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    g++ \
    libopus-dev \
    libopusfile-dev \
&& rm -rf /var/lib/apt/lists/*

WORKDIR /src/yomiko

# download go modules
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

# build servers
COPY ./audio ./audio
COPY ./bot ./bot
COPY ./cmd ./cmd
COPY ./ent ./ent
COPY ./ssml ./ssml
COPY ./tts ./tts
COPY ./misc ./misc
RUN go install ./cmd/yomiko

#========================================
# Runtime
FROM debian:trixie-slim AS runtime

# install packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libopus0 \
    libopusfile0 \
&& rm -rf /var/lib/apt/lists/*

RUN mkdir -p /etc/yomiko /usr/var/lib/yomiko

ENV YOMIKO_TOKEN=""
ENV YOMIKO_CREDENTIALS_JSON=""
ENV YOMIKO_CREDENTIALS_FILE="/etc/yomiko/credentials.json"
ENV YOMIKO_DATABASE_PATH="/usr/var/lib/yomiko/yomiko.db"

COPY --from=builder /go/bin/yomiko /usr/bin/yomiko
COPY ./misc/docker/config.toml /etc/yomiko/config.toml

ENTRYPOINT ["yomiko"]
CMD ["run", "-c", "/etc/yomiko/config.toml"]
