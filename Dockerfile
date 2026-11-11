FROM node:lts-alpine as febuild
WORKDIR /work

COPY web/desktop .

RUN yarn && yarn build


FROM golang:1.27.3-alpine as serverbuild
ARG VERSION
WORKDIR /work
COPY . .
COPY --from=febuild /work/dist /work/server/internal/api/dist
RUN apk update && apk add git
RUN cd /work/server && go build -ldflags "-s -w -X 'main.version=${VERSION}' -X 'main.goVersion=$(go version)' -X 'main.gitHash=$(git show -s --format=%H)' -X 'main.buildTime=$(TZ=UTC-8 date +%Y-%m-%d" "%H:%M:%S)'" -o pmail ./cmd/pmail
RUN cd /work/server/hooks/wasm_plugins && for p in spam_block wechat_push; do mkdir -p output/$p && CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o output/$p/plugin.wasm ./cmd/$p && cp manifests/$p.json output/$p/manifest.json; done


FROM alpine

WORKDIR /work

# 设置时区
RUN apk add --no-cache tzdata \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    &&rm -rf /var/cache/apk/* /tmp/* /var/tmp/* $HOME/.cache


COPY --from=serverbuild /work/server/pmail .
COPY --from=serverbuild /work/server/hooks/wasm_plugins/output/spam_block ./plugins/wasm/spam_block/
COPY --from=serverbuild /work/server/hooks/wasm_plugins/output/wechat_push ./plugins/wasm/wechat_push/

EXPOSE 25 80 110 443 465 587 995 993

CMD /work/pmail
