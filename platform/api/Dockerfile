FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN mkdir -p /out/data/media

# Test-only GitHub OAuth double for the Compose e2e check (compose.e2e.yaml). It is a separate
# target and never part of the default (last) stage below.
FROM build AS fakegithub-build
RUN CGO_ENABLED=0 go build -trimpath -o /out/fakegithub ./cmd/fakegithub
FROM gcr.io/distroless/static-debian12:nonroot AS fakegithub
COPY --from=fakegithub-build /out/fakegithub /fakegithub
ENTRYPOINT ["/fakegithub"]

# Distroless: no shell or curl; the binary's own "healthcheck" command probes liveness.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
# Owned by nonroot so a fresh media_data volume inherits writable permissions.
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 CMD ["/server", "healthcheck"]
ENTRYPOINT ["/server"]
CMD ["serve"]
