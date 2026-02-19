# downguy
#
# Two stages. The build stage has the toolchain and the module cache; the
# runtime stage has the binary, a writable data directory and nothing else.
# There is no cgo anywhere in this program, so the runtime image needs no libc
# beyond what the distroless base carries.

FROM golang:1.23.5-bookworm AS build

WORKDIR /src

# The module files go in on their own so that a change to the source does not
# invalidate the dependency download layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG RELEASE=dev
ARG COMMIT=
ARG BUILT_AT=

RUN CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w \
            -X github.com/glinskevxer/downguy/internal/version.Release=${RELEASE} \
            -X github.com/glinskevxer/downguy/internal/version.Commit=${COMMIT} \
            -X github.com/glinskevxer/downguy/internal/version.BuiltAt=${BUILT_AT}" \
        -o /out/downguy ./cmd/downguy

# Prove the binary runs before it leaves the stage that can still debug it.
RUN /out/downguy version && /out/downguy help


FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /data

COPY --from=build /out/downguy /usr/local/bin/downguy

# The service is read only over http and writes only to its own database, so it
# runs as the base image's unprivileged account and owns nothing else.
USER nonroot:nonroot

ENV DOWNGUY_DATABASE=/data/downguy.db \
    DOWNGUY_ADDRESS=0.0.0.0:8471

EXPOSE 8471

# The image has no shell, so the health check runs the binary itself rather
# than reaching for curl.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/downguy", "serve", "-check"]

ENTRYPOINT ["/usr/local/bin/downguy"]
CMD ["serve"]
