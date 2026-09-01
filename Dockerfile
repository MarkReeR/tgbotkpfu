# The SQLite driver is pure Go, so the whole thing builds with CGO disabled and
# ships as a single static binary on a distroless base.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencies first, so edits to the source do not invalidate the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/bot ./cmd

# An empty data directory to copy into the final image. Distroless has no shell,
# so it cannot be created there - and it has to exist, owned by nonroot, before
# a named volume is mounted over it: Docker seeds a fresh volume from the image,
# ownership included. Without this the container cannot write its own database.
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# /app/data holds the SQLite file and the logs; mount a volume over it to keep
# the chosen groups across container restarts.
COPY --from=build --chown=nonroot:nonroot /out/bot /app/bot
COPY --from=build --chown=nonroot:nonroot /out/data /app/data

ENV DATABASE_PATH=/app/data/Database.db \
    LOG_DIR=/app/data/logs \
    CONFIG_PATH=/app/config.ini \
    TZ=Europe/Moscow

USER nonroot
ENTRYPOINT ["/app/bot"]
