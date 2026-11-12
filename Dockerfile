# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS serverbuild
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_HASH=unknown
ARG BUILD_TIME=unknown
ARG GOPROXY=https://proxy.golang.org,direct
WORKDIR /work
COPY server/go.mod server/go.sum ./server/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd server && GOPROXY=${GOPROXY} go mod download
COPY server/ ./server/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd server && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w -X 'main.version=${VERSION}' -X 'main.goVersion=$(go version)' -X 'main.gitHash=${GIT_HASH}' -X 'main.buildTime=${BUILD_TIME}'" \
    -o pmail ./cmd/pmail

FROM alpine AS runtime
ARG VERSION=dev
LABEL org.opencontainers.image.title="PMail" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.description="Self-hosted mail server with SMTP/IMAP/POP3 and a web admin UI"
WORKDIR /work
RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone
COPY --from=serverbuild /work/server/pmail ./pmail
EXPOSE 25 80 110 443 465 587 993 995
CMD ["/work/pmail"]
