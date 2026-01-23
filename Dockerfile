FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ccproxy ./cmd/ccproxy
COPY cmd/ccdebug ./cmd/ccdebug
COPY cmd/ccclipd ./cmd/ccclipd
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o ccproxy ./cmd/ccproxy
RUN CGO_ENABLED=0 go build -o ccdebug ./cmd/ccdebug
RUN CGO_ENABLED=0 go build -o ccclipd ./cmd/ccclipd

FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    netcat-openbsd \
    gosu \
    openssh-client \
    jq \
    ripgrep \
    make \
    build-essential \
    python3 \
    vim-tiny \
    xvfb \
    xclip \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://claude.ai/install.sh | bash

# Copy binaries from builder
COPY --from=builder /build/ccproxy /usr/local/bin/ccproxy
COPY --from=builder /build/ccdebug /usr/local/bin/ccdebug
COPY --from=builder /build/ccclipd /usr/local/bin/ccclipd

# Create hijacker directory (will be populated at runtime by ccproxy --setup)
# This gets chown'd to the claude user later after user is created
RUN mkdir -p /opt/ccyolo/bin

# Create non-root user (use different UID/GID since 1000 is taken by node user)
ARG USERNAME=claude
ARG USER_UID=1001
ARG USER_GID=${USER_UID}

RUN groupadd --gid ${USER_GID} ${USERNAME} \
    && useradd --uid ${USER_UID} --gid ${USER_GID} -m ${USERNAME}

# Set up claude for the non-root user
# Create ~/.local/bin and copy claude there (expected by native install detection)
RUN mkdir -p /home/${USERNAME}/.local/bin \
    && cp /root/.local/bin/claude /home/${USERNAME}/.local/bin/claude \
    && chown -R ${USERNAME}:${USERNAME} /home/${USERNAME}/.local

# Create necessary directories and set ownership
RUN mkdir -p /workspace /home/${USERNAME}/.claude \
    && chown -R ${USERNAME}:${USERNAME} /workspace /home/${USERNAME}/.claude /opt/ccyolo

# Set PATH for the claude user (hijacker dir first, then local bin)
ENV PATH="/opt/ccyolo/bin:/home/${USERNAME}/.local/bin:${PATH}"

# Set DISPLAY for xclip to work with Xvfb
ENV DISPLAY=:99

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
