# Makefile do MegaBrain (módulo Lua + backend Go).
# Rode a partir desta pasta: make <alvo>

ENGINE   := engine
BIN      := $(ENGINE)/megabrain-engine
DATA_DIR ?= $(HOME)/.local/share/otclient/megabrain

.PHONY: help all build cross perms run vet fmt test check-lua clean

help:
	@echo "Alvos disponíveis:"
	@echo "  make build      Compila o backend para a plataforma atual"
	@echo "  make cross      Cross-compila linux/windows/darwin (build/build.sh)"
	@echo "  make perms      Dá permissão de execução ao binário"
	@echo "  make run        Compila e executa (DATA_DIR=$(DATA_DIR))"
	@echo "  make vet        go vet"
	@echo "  make fmt        gofmt -w"
	@echo "  make test       go test"
	@echo "  make check-lua  Verifica a sintaxe dos .lua (luac -p)"
	@echo "  make clean      Remove binário e artefatos de build"

all: build

build:
	cd $(ENGINE) && go build -o megabrain-engine .
	@chmod +x $(BIN) 2>/dev/null || true

cross:
	cd $(ENGINE) && sh build/build.sh
	@chmod +x $(BIN) 2>/dev/null || true

perms:
	@chmod +x $(BIN) 2>/dev/null || true
	@echo "Permissão aplicada em $(BIN)"

run: build
	$(BIN) -data-dir $(DATA_DIR)

vet:
	cd $(ENGINE) && go vet ./...

fmt:
	cd $(ENGINE) && gofmt -w .

test:
	cd $(ENGINE) && go test ./...

check-lua:
	luac -p megabrain.lua core/*.lua && echo "Lua OK"

clean:
	rm -f $(BIN)
	rm -rf $(ENGINE)/build/out
