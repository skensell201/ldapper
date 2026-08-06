.PHONY: test lint integration tidy build front demo demo-down release-build

# VERSION is what the status bar shows. A local build says "dev" unless told
# otherwise, because claiming a release it is not would mislead a bug report.
VERSION ?= dev
LDFLAGS := -X github.com/skensell201/ldapper/app.version=$(VERSION)

test: front
	go test -race ./...

# The interface has tests of its own, and they catch the class of fault that
# no Go test can: a pane that crashes on real data and takes the window with it.
front:
	npm --prefix frontend ci --silent
	npm --prefix frontend test

lint:
	go vet ./...
	golangci-lint run

integration:
	docker compose -f test/integration/docker-compose.yml up -d --wait
	go test -tags=integration ./test/integration/... -v
	docker compose -f test/integration/docker-compose.yml down -v

build:
	go build ./...

# release-build produces the same binary the release workflow does, for
# checking a stamp locally before cutting a tag.
release-build:
	wails build -clean -ldflags "$(LDFLAGS)"

tidy:
	go mod tidy

# demo brings up a directory worth exploring, saves a connection for it, and
# opens the application already pointed at it. Everything it touches lives
# under build/demo, so a real Ldapper installation is never disturbed.
#
# The connection binds anonymously, so there is nothing to type: the directory
# allows reading that way, which is what most do.
demo:
	docker compose -f dev/directory/docker-compose.yml up -d --wait
	@mkdir -p build/demo
	@printf '%s\n' '[{"id":"demo","name":"Example Corporation","host":"localhost","port":4389,"encryption":"none","bindMethod":"anonymous","rememberPassword":false}]' > build/demo/connections.json
	wails build -ldflags "$(LDFLAGS)"
	@echo
	@echo "  Ldapper is opening against the demo directory."
	@echo "  The connection is already saved — press Connect, and nothing else."
	@echo
	@LDAPPER_CONFIG_DIR="$(PWD)/build/demo" nohup ./build/bin/Ldapper.app/Contents/MacOS/Ldapper >/dev/null 2>&1 &

demo-down:
	docker compose -f dev/directory/docker-compose.yml down -v
