FROM alpine:latest

WORKDIR /app/web 

# Release tag to download when ./dist has no binary ("latest" or e.g. "v2.3.1").
ARG CODG_VERSION=latest
ARG CODG_BIN=codg_linux_amd64

# Copy ./dist; .dockerignore filters it to only codg_linux_amd64_web, so
# this works even when ./dist is empty (no binary yet).
COPY dist/ ./dist/
# COPY --from=0 /app/web/codg ./dist/codg_linux_amd64
# COPY --from=0 /app/web/conf ./conf

# Fallback: if ./dist does not have the binary, download it from the GitHub
# releases. Try the raw binary asset first, then the .zip asset.
RUN if [ ! -f "./dist/${CODG_BIN}" ]; then \
        apk add --no-cache ca-certificates && \
        if [ "${CODG_VERSION}" = "latest" ]; then \
            URL="https://github.com/vcaesar/codg/releases/latest/download"; \
        else \
            URL="https://github.com/vcaesar/codg/releases/download/${CODG_VERSION}"; \
        fi && \
        ( wget -q -O "./dist/${CODG_BIN}" "${URL}/${CODG_BIN}" || \
          ( rm -f "./dist/${CODG_BIN}" && \
            wget -q -O "/tmp/${CODG_BIN}.zip" "${URL}/${CODG_BIN}.zip" && \
            unzip -o "/tmp/${CODG_BIN}.zip" -d ./dist/ && \
            rm -f "/tmp/${CODG_BIN}.zip" ) ); \
    fi && \
    chmod +x "./dist/${CODG_BIN}" && \
    ln -sf "dist/${CODG_BIN}" ./codg

EXPOSE 4096
ENV PORT=4096
ENV HOSTNAME="127.0.0.1"

# Start the web UI server. --skip-build: no npm/ui sources in the image,
# the release binary ships pre-built frontend assets.
CMD ["./codg", "web", "--skip-build", "-p", "4096"]
