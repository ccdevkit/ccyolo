FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    netcat-openbsd \
    gosu \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL https://claude.ai/install.sh | bash

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

# Create necessary directories
RUN mkdir -p /workspace /home/${USERNAME}/.claude \
    && chown -R ${USERNAME}:${USERNAME} /workspace /home/${USERNAME}/.claude

# Set PATH for the claude user
ENV PATH="/home/${USERNAME}/.local/bin:${PATH}"

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace

ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
