FROM alpine:3.19

LABEL org.opencontainers.image.source='https://github.com/neilotoole/sq'
LABEL org.opencontainers.image.description='sq data wrangler, on Alpine Linux'
LABEL org.opencontainers.image.licenses=MIT

WORKDIR /root

RUN apk add --update --no-cache curl gnupg wget bash bash-completion zsh \
    tzdata git jq coreutils utmps vim nano \
    sqlite postgresql-client mysql-client

# This Dockerfile is built by GoReleaser (see .goreleaser-docker.yml). GoReleaser
# assembles a build context containing the freshly-built sq binary at the context
# root, plus the helper scripts below (declared as `extra_files`, which preserve
# their repo-relative path). Building from the just-built binary means the image
# always matches this exact release; there's no fetch of the "latest" GitHub
# release at build time.

# Microsoft SQL Server Tools are slightly trickier to install.
COPY packages/docker/alpine-mssql-tools-install.sh ./alpine-mssql-tools-install.sh
RUN chmod +x ./alpine-mssql-tools-install.sh
RUN ./alpine-mssql-tools-install.sh
ENV PATH=$PATH:/opt/mssql-tools18/bin

# Install sq from the binary GoReleaser placed at the build-context root.
# `sq completion bash` doubles as a liveness check that the copied binary runs.
# We deliberately avoid `sq version` here: it makes a best-effort (non-fatal)
# network call to check for updates, which would reintroduce a build-time
# network dependency — the very thing this approach removes.
COPY sq /usr/local/bin/sq
RUN chmod +x /usr/local/bin/sq \
    && mkdir -p /etc/bash_completion.d/ \
    && sq completion bash > /etc/bash_completion.d/sq \
    && echo "source /etc/bash/bash_completion.sh" >> /etc/bash/bashrc

# Configure zsh.
COPY packages/docker/sq.zsh-theme ./sq.zsh-theme
COPY packages/docker/configure-oh-my-zsh.sh ./configure-oh-my-zsh.sh
RUN chmod +x ./configure-oh-my-zsh.sh
RUN ./configure-oh-my-zsh.sh

# Clean up.
RUN rm ./alpine-mssql-tools-install.sh ./configure-oh-my-zsh.sh

COPY packages/docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
