# syntax=docker/dockerfile:1.7

FROM golang:1.27rc2-alpine3.24@sha256:dcbb18cc5fa1082364dc6aa95224b6b55429d09cbb9631a053d8064c1c367300 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG SOURCE_URL=
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
      -o /out/ecs-exporter ./cmd/ecs-exporter

FROM scratch
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG SOURCE_URL=
LABEL org.opencontainers.image.title="Dell ECS Metrics Exporter" \
      org.opencontainers.image.description="Prometheus exporter and inventory API for Dell ECS" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/ecs-exporter /ecs-exporter
COPY profiles /profiles
COPY LICENSE /LICENSE
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/ecs-exporter"]
CMD ["-config=/etc/ecs-exporter/config.yaml", "-profiles-dir=/profiles"]
