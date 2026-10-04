# Image for the Redmine MCP server, built by GoReleaser (dockers_v2). The
# build context holds the prebuilt binaries under <os>/<arch>/; nothing is
# compiled here, so no RUN steps and no emulation are needed.
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/redmine /usr/local/bin/redmine

# No keyring or self-update inside a container: credentials come from
# REDMINE_* variables or a mounted ~/.redmine-cli.yaml.
ENV REDMINE_NO_KEYRING=1 \
    REDMINE_NO_UPDATE_CHECK=1

# Only used with REDMINE_MCP_HTTP=0.0.0.0:8080 (or --http 0.0.0.0:8080);
# stdio is the default transport.
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/redmine", "mcp", "serve"]
