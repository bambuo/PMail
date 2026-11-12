# syntax=docker/dockerfile:1

FROM node:24-alpine AS febuild
WORKDIR /work
RUN npm install --global bun@1.4.4
COPY web/desktop/package.json web/desktop/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/desktop/ ./
RUN bun run build

FROM golang:1.27.3-alpine AS serverbuild
ARG VERSION=dev
ARG GIT_HASH=unknown
ARG BUILD_TIME=unknown
WORKDIR /work
COPY server/go.mod server/go.sum ./server/
RUN cd server && go mod download
COPY server/ ./server/
COPY --from=febuild /work/dist ./server/internal/api/http/dist
RUN cd server && CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X 'main.version=${VERSION}' -X 'main.goVersion=$(go version)' -X 'main.gitHash=${GIT_HASH}' -X 'main.buildTime=${BUILD_TIME}'" \
    -o pmail ./cmd/pmail
RUN cd server/hooks/wasm_plugins && for p in spam_block wechat_push; do \
        mkdir -p output/$p && \
        CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o output/$p/plugin.wasm ./cmd/$p && \
        cp manifests/$p.json output/$p/manifest.json; \
    done

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
COPY --from=serverbuild /work/server/hooks/wasm_plugins/output/spam_block ./plugins/wasm/spam_block/
COPY --from=serverbuild /work/server/hooks/wasm_plugins/output/wechat_push ./plugins/wasm/wechat_push/
EXPOSE 25 80 110 443 465 587 993 995
CMD ["/work/pmail"]
