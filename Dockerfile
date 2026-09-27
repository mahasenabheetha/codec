# Release image, built by GoReleaser (dockers_v2 in .goreleaser.yaml)
# from the binaries it has already compiled; it does not build Go here.
# The context holds one binary per platform at <os>/<arch>/codec.
#
#   docker run --rm -p 127.0.0.1:8765:8765 -v "$PWD:/work:ro" ghcr.io/mahasenabheetha/codec
#
# distroless/static: no shell or package manager, CA certificates and a
# non-root user only. codec is fully static (CGO_ENABLED=0).
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/codec /usr/bin/codec

# Mount the repository here, read-only. codec never writes to it; its
# own settings go to the non-root user's home inside the container.
WORKDIR /work
EXPOSE 8765

# 0.0.0.0 so the published port reaches it. Publish it on the host as
# 127.0.0.1:8765:8765 so only this machine can connect. File changes are
# picked up by polling (automatic inside containers).
ENTRYPOINT ["/usr/bin/codec", "serve", "--host", "0.0.0.0", "--root", "/work"]
