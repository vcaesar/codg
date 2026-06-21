FROM debian:stable-slim
# FROM ubuntu:latest

WORKDIR /app/web

# Release tag to download when ./dist has no binary ("latest" or e.g. "v2.3.1").
ARG CODG_VERSION=latest
ARG CODG_BIN=codg_linux_amd64

# Runtime deps (the release binary is a static pure-Go build, so no libc shim
# is needed):
#  - ca-certificates : HTTPS calls to model/provider APIs and to GitHub releases.
#  - unzip           : extracts the downloaded release .zip below.
#  - wget            : downloads the release asset (not preinstalled on slim).
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates unzip wget && \
    rm -rf /var/lib/apt/lists/*

# Copy any locally-built binary from ./dist. The repo ships an empty
# dist/.gitkeep so this COPY succeeds even when no binary exists yet; in that
# case the RUN below downloads the release.
# COPY dist/ ./dist/

# Fallback: if ./dist does not have the binary, download it from the GitHub
# releases. The releases only publish a ".zip" asset (e.g.
# "codg_linux_amd64.zip"); there is NO raw-binary asset, so requesting
# "${CODG_BIN}" directly always 404s. We therefore fetch the .zip and unzip it
# (the archive contains a single file named exactly "${CODG_BIN}").
RUN if [ "${CODG_VERSION}" = "latest" ]; then \
        URL="https://github.com/vcaesar/codg/releases/latest/download"; \
    else \
        URL="https://github.com/vcaesar/codg/releases/download/${CODG_VERSION}"; \
    fi && \
    echo "Downloading ${URL}/${CODG_BIN}.zip" && \
    wget -q -O "/tmp/${CODG_BIN}.zip" "${URL}/${CODG_BIN}.zip" && \
    unzip -o "/tmp/${CODG_BIN}.zip" -d ./dist/ && \
    rm -f "/tmp/${CODG_BIN}.zip" && \
    # Fail the build loudly if the binary is still missing (bad ARG, 404, etc.)
    # instead of producing an image that crash-loops at runtime.
    test -f "./dist/${CODG_BIN}" || { echo "ERROR: ./dist/${CODG_BIN} not found"; exit 1; } && \
    chmod +x "./dist/${CODG_BIN}" && \
    ln -sf "dist/${CODG_BIN}" ./codg

EXPOSE 4096
ENV PORT=4096
# Bind to all interfaces. 127.0.0.1 only listens on the container's own
# loopback, so the published port would be unreachable from the host.
ENV HOSTNAME="127.0.0.1"

# Start the web UI server. The release binary ships pre-built frontend assets.
CMD ["./codg", "web", "-p", "4096"]