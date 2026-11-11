.PHONY: build clean build_fe build_server wasm_plugins package test test_mysql test_postgres run run_dev check_arch

build: build_fe build_server wasm_plugins package

clean:
	rm -rf output


build_fe:
	cd web/desktop && bun install --frozen-lockfile && bun run build
	rm -rf server/internal/api/dist
	cd server && cp -rf ../web/desktop/dist internal/api

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



package: clean
	mkdir output
	mv server/pmail* output/
	mkdir output/config
	mkdir output/plugins
	cp -r server/config/dkim output/config/
	cp -r server/config/ssl output/config/
	cp -r server/config/config.json output/config/
	mkdir -p output/plugins/wasm
	cp -r server/hooks/wasm_plugins/output/spam_block output/plugins/wasm/
	cp -r server/hooks/wasm_plugins/output/wechat_push output/plugins/wasm/
	cp README.md output/

test:
	export setup_port=17888 && cd server && export PMail_ROOT=$(CURDIR)/server/ && go test -v -p 1 ./...

test_mysql:
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
