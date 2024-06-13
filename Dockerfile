################################
# Build
FROM golang:1.22-bookworm AS builder

ENV DEBIAN_FRONTEND noninteractive

# install packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    g++ \
    libopus-dev \
    libopusfile-dev \
&& rm -rf /var/lib/apt/lists/*

# set GOTOOLCHAIN env
ENV GOTOOLCHAIN=auto

WORKDIR /src/yomiko

# download go modules
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

# build servers
COPY ./audio ./audio
COPY ./bot ./bot
COPY ./cmd ./cmd
COPY ./tts ./tts
COPY ./misc ./misc
RUN go install ./cmd/yomiko

#========================================
# whisper server
FROM debian:bookworm-slim as runtime

# install packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libopus0 \
    libopusfile0 \
&& rm -rf /var/lib/apt/lists/*

ENV YOMIKO_TOKEN ""
ENV YOMIKO_CREDENTIALS_JSON ""
ENV YOMIKO_CREDENTIALS_FILE "/etc/yomiko/credentials.json"

COPY --from=builder /go/bin/yomiko /usr/bin/yomiko
COPY ./misc/docker/config.toml /etc/yomiko/config.toml

ENTRYPOINT ["yomiko"]
CMD ["run", "-c", "/etc/yomiko/config.toml"]
