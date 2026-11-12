.PHONY: build clean build_fe build_server wasm_plugins plugin_tgz package docker_build test test_e2e test_mysql test_postgres run run_dev check_arch

build: build_fe build_server package

clean:
	rm -rf output


build_fe:
	cd web/desktop && bun install --frozen-lockfile && bun run build
	rm -rf server/internal/api/http/dist
	cd server && cp -rf ../web/desktop/dist internal/api/http

build_server:
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X 'main.goVersion=$(go version)' -X 'main.gitHash=$(git show -s --format=%H)' -X 'main.buildTime=$(TZ=UTC-8 date +%Y-%m-%d" "%H:%M:%S)'" -o pmail_linux_amd64  ./cmd/pmail
	cd server && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X 'main.goVersion=$(go version)' -X 'main.gitHash=$(git show -s --format=%H)' -X 'main.buildTime=$(TZ=UTC-8 date +%Y-%m-%d" "%H:%M:%S)'" -o pmail_windows_amd64.exe  ./cmd/pmail
	cd server && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X 'main.goVersion=$(go version)' -X 'main.gitHash=$(git show -s --format=%H)' -X 'main.buildTime=$(TZ=UTC-8 date +%Y-%m-%d" "%H:%M:%S)'" -o pmail_mac_amd64  ./cmd/pmail
	cd server && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X 'main.goVersion=$(go version)' -X 'main.gitHash=$(git show -s --format=%H)' -X 'main.buildTime=$(TZ=UTC-8 date +%Y-%m-%d" "%H:%M:%S)'" -o pmail_mac_arm64  ./cmd/pmail

wasm_plugins:
	cd server/hooks/wasm_plugins && for p in spam_block wechat_push debug; do \
		mkdir -p output/$$p; \
		CGO_ENABLED=0 GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o output/$$p/plugin.wasm ./cmd/$$p; \
		cp manifests/$$p.json output/$$p/manifest.json; \
	done

plugin_tgz: wasm_plugins
	cd server/hooks/wasm_plugins/output && for p in spam_block wechat_push debug; do COPYFILE_DISABLE=1 tar -czf $$p.tgz -C $$p .; done
	@echo "插件包已生成：server/hooks/wasm_plugins/output/*.tgz"



package: clean
	mkdir output
	mv server/pmail* output/
	mkdir output/config
	cp -r deploy/config/dkim output/config/
	cp -r deploy/config/ssl output/config/
	cp -r deploy/config/config.json output/config/
	cp README.md output/

docker_build:
	@test -d server/internal/api/http/dist || { echo "缺少前端产物：先执行 make build_fe"; exit 1; }
	docker buildx build \
		--platform $${PLATFORM:-linux/amd64} \
		--build-arg VERSION=$$(git describe --tags --always 2>/dev/null || echo dev) \
		--build-arg GIT_HASH=$$(git rev-parse HEAD 2>/dev/null || echo unknown) \
		--build-arg BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
		--build-arg GOPROXY=$${GOPROXY:-https://proxy.golang.org,direct} \
		-t $${IMAGE:-pmail:latest} --load .

test_fixtures:
	mkdir -p server/config
	cp -R deploy/config/. server/config/

test: test_fixtures
	export setup_port=17888 && cd server && export PMail_ROOT=$(CURDIR)/server/ && go test -v -p 1 ./...

test_e2e: test_fixtures
	export setup_port=17888 && cd server && export PMail_ROOT=$(CURDIR)/server/ && go test -tags e2e -v -p 1 ./...

test_mysql: test_fixtures
	export setup_port=17888 && cd server && export PMail_ROOT=$(CURDIR)/server/ && go test -args "mysql" -v -p 1 ./...

test_postgres:
	export setup_port=17888 && cd server && export PMail_ROOT=$(CURDIR)/server/ && go test -args "postgres" -v -p 1 ./...

# Run from the module root so relative plugin and certificate paths stay valid.
run:
	cd server && PMail_ROOT="$(CURDIR)/server/" go run ./cmd/pmail

run_dev:
	cd server && PMail_ROOT="$(CURDIR)/server/" go run ./cmd/pmail dev

check_arch:
	node scripts/check-architecture.mjs
