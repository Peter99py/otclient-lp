# =============================================================================
# Makefile - OTClient build para Linux
# =============================================================================
# Uso:
#   make help            - mostra ajuda
#   make install-deps    - instala deps do Fedora + Perl + Autotools no distrobox
#   make vcpkg           - baixa vcpkg e instala deps C++ do projeto
#   make configure       - roda cmake --preset linux-release
#   make build           - compila o otclient
#   make dist            - copia binario + assets para build/dist/
#   make all             - install-deps + vcpkg + build + dist
#   make clean           - remove build/
#   make clean-all       - remove build/ + cache do vcpkg
#   make run             - roda o client de build/dist/
# =============================================================================

# --- Configuracoes ------------------------------------------------------------
DISTROBOX_NAME  ?= fedora
VCPKG_COMMIT    ?= $(shell python3 -c 'import json; print(json.load(open("vcpkg.json"))["builtin-baseline"])' 2>/dev/null || echo "9eec002df1cd109f8f104cd966a763daf9751489")
VCPKG_ROOT      := $(HOME)/vcpkg
VCPKG_INSTALLED := $(HOME)/vcpkg_installed
VCPKG_MANIFEST  := $(HOME)/vcpkg-manifest
BUILD_PRESET     = linux-release
BIN_NAME        ?= silverlakeot
SOURCE_DIR       = $(CURDIR)
BUILD_DIR        = $(SOURCE_DIR)/build
DIST_DIR         = $(BUILD_DIR)/dist
CMAKE_BUILD_TYPE = RelWithDebInfo

# --- Variaveis para passar ao distrobox --------------------------------------
DISTROBOX_ENV = VCPKG_ROOT=$(VCPKG_ROOT) VCPKG_INSTALLED_DIR=$(VCPKG_INSTALLED)

# Helpers ---------------------------------------------------------------------
DB := distrobox enter $(DISTROBOX_NAME) --

# Comando base para executar dentro do distrobox
define db_run
	@echo ">> [distrobox:$(DISTROBOX_NAME)] $(1)"
	@$(DB) bash -c '$(1)'
endef

.PHONY: help install-deps vcpkg configure build dist all build-fedora build-ubuntu run clean clean-all check-distrobox

# --- Ajuda --------------------------------------------------------------------
help:
	@echo "OTClient Linux Build"
	@echo
	@echo "Alvos disponiveis:"
	@echo "  make all           - Build completo do zero (install-deps + vcpkg + configure + build + dist)"
	@echo "  make install-deps  - Instala compilador e libs do sistema no distrobox"
	@echo "  make vcpkg         - Baixa vcpkg e compila dependencias C++"
	@echo "  make configure     - Roda o CMake"
	@echo "  make build         - Compila o otclient"
	@echo "  make dist          - Cria pasta dist/ com binario e assets"
	@echo "  make run           - Executa o client compilado"
	@echo "  make clean         - Limpa pasta build/"
	@echo "  make clean-all     - Limpa build/ e cache do vcpkg"
	@echo "  make build-ubuntu  - Build nativo no Ubuntu (sem distrobox, usado no GitHub Actions)"
	@echo
	@echo "Variaveis:"
	@echo "  DISTROBOX_NAME=$(DISTROBOX_NAME)"
	@echo "  BUILD_PRESET=$(BUILD_PRESET)"
	@echo "  CMAKE_BUILD_TYPE=$(CMAKE_BUILD_TYPE)"

# --- 1. Instalar dependencias do Fedora no distrobox -------------------------
install-deps:
	@echo ">> Instalando dependencias do Fedora no distrobox..."
	$(call db_run, sudo dnf install -y \
		cmake ninja-build gcc gcc-c++ make git curl wget pkg-config \
		libstdc++ libstdc++-devel \
		libX11-devel libXcursor-devel libXi-devel libXinerama-devel libXrandr-devel \
		libGL-devel libGLU-devel mesa-libGL-devel mesa-libGLU-devel \
		openssl-devel zlib-devel bzip2-devel xz-devel libarchive-devel \
		perl-IPC-Cmd perl-File-Compare perl-Time-Piece perl-IO \
		perl-Thread perl-Thread-Queue perl-Scalar-Util perl-List-Util perl-Storable \
		autoconf automake libtool libtool-ltdl-devel autoconf-archive)

# --- 2. Configurar vcpkg e instalar deps do projeto ---------------------------
vcpkg: $(VCPKG_ROOT)/vcpkg
	@echo ">> Instalando dependencias C++ via vcpkg (~15 min na primeira vez)..."
	$(call db_run, mkdir -p $(VCPKG_MANIFEST) && \
		cp $(SOURCE_DIR)/vcpkg.json $(VCPKG_MANIFEST)/ && \
		cd $(VCPKG_MANIFEST) && \
		$(VCPKG_ROOT)/vcpkg install \
			--x-manifest-root=$(VCPKG_MANIFEST) \
			--x-install-root=$(VCPKG_INSTALLED) \
			--triplet=x64-linux \
			--host-triplet=x64-linux)

# Clone e bootstrap do vcpkg (roda dentro do distrobox)
$(VCPKG_ROOT)/vcpkg:
	@echo ">> Clonando vcpkg ($(VCPKG_COMMIT))..."
	$(call db_run, if [ ! -d "$(VCPKG_ROOT)" ]; then git clone https://github.com/microsoft/vcpkg.git $(VCPKG_ROOT); fi && \
		cd $(VCPKG_ROOT) && \
		git fetch origin $(VCPKG_COMMIT) && \
		git checkout $(VCPKG_COMMIT) && \
		./bootstrap-vcpkg.sh -disableMetrics)

# --- 3. Configurar build com CMake -------------------------------------------
configure:
	@echo ">> Configurando build (cmake --preset $(BUILD_PRESET))..."
	$(call db_run, cd $(SOURCE_DIR) && \
		rm -rf build/$(BUILD_PRESET) && \
		$(DISTROBOX_ENV) cmake --preset $(BUILD_PRESET) \
			-DCMAKE_BUILD_TYPE=$(CMAKE_BUILD_TYPE) \
			-DTOGGLE_BIN_FOLDER=ON \
			-DOPTIONS_ENABLE_IPO=OFF \
			-DOTCLIENT_BUILD_TESTS=OFF \
			-DBUILD_STATIC_LIBRARY=ON \
			-DVCPKG_MANIFEST_INSTALL=OFF \
			-DVCPKG_INSTALLED_DIR=$(VCPKG_INSTALLED))

# --- 4. Compilar --------------------------------------------------------------
build:
	@echo ">> Compilando otclient..."
	$(call db_run, cd $(SOURCE_DIR) && \
		$(DISTROBOX_ENV) cmake --build --preset $(BUILD_PRESET) --target otclient -j\$$(nproc))

# --- 5. Empacotar binario + assets -------------------------------------------
dist: build
	@echo ">> Empacotando binario + assets em $(DIST_DIR)/..."
	@mkdir -p $(DIST_DIR)
	@cp $(BUILD_DIR)/$(BUILD_PRESET)/bin/silverlakeot $(DIST_DIR)/$(BIN_NAME) 2>/dev/null || cp $(BUILD_DIR)/$(BUILD_PRESET)/bin/otclient $(DIST_DIR)/$(BIN_NAME)
	@chmod +x $(DIST_DIR)/$(BIN_NAME)
	@cp -r data mods modules init.lua otclientrc.lua $(DIST_DIR)/ 2>/dev/null || true
	@[ -f cacert.pem ] && cp cacert.pem $(DIST_DIR)/ || true
	@[ -f config.ini ] && cp config.ini $(DIST_DIR)/ || true
	@echo
	@echo ">> Pronto! Para rodar:"
	@echo "   cd $(DIST_DIR) && ./$(BIN_NAME)"
	@echo
	@ls -la $(DIST_DIR)/$(BIN_NAME)
	@file $(DIST_DIR)/$(BIN_NAME) 2>/dev/null || true

# --- 6. Build completo (atalho) ----------------------------------------------
all: install-deps vcpkg configure build dist

# --- 6.1 Build completo para Fedora (via distrobox) -------------------------
build-fedora: all

# --- 7. Rodar o client --------------------------------------------------------
run: dist
	$(DB) bash -c 'cd $(DIST_DIR) && ./$(BIN_NAME)'

# --- Limpeza ------------------------------------------------------------------
clean:
	@echo ">> Removendo $(BUILD_DIR)/..."
	@rm -rf $(BUILD_DIR)

clean-all: clean
	@echo ">> Removendo cache do vcpkg..."
	$(call db_run, rm -rf $(VCPKG_INSTALLED) $(VCPKG_ROOT) $(VCPKG_MANIFEST) \$$HOME/.cache/vcpkg)

# --- 8. Build para Ubuntu (sem distrobox) ------------------------------------
build-ubuntu:
	@echo ">> Instalando dependencias do Ubuntu nativamente..."
	@sudo apt-get update
	@sudo apt-get install -y \
		cmake ninja-build gcc g++ make git curl wget pkg-config \
		libx11-dev libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev \
		libgl1-mesa-dev libglu1-mesa-dev \
		libssl-dev zlib1g-dev libbz2-dev liblzma-dev libarchive-dev \
		perl \
		autoconf automake libtool libltdl-dev autoconf-archive

	@echo ">> Clonando e configurando vcpkg ($(VCPKG_COMMIT))..."
	@if [ ! -d "$(VCPKG_ROOT)" ]; then \
		git clone https://github.com/microsoft/vcpkg.git "$(VCPKG_ROOT)"; \
	fi
	@cd "$(VCPKG_ROOT)" && \
		git fetch origin $(VCPKG_COMMIT) && \
		git checkout $(VCPKG_COMMIT) && \
		./bootstrap-vcpkg.sh -disableMetrics

	@mkdir -p "$(VCPKG_MANIFEST)"
	@cp vcpkg.json "$(VCPKG_MANIFEST)/"
	@cd "$(VCPKG_MANIFEST)" && \
		"$(VCPKG_ROOT)/vcpkg" install \
			--x-manifest-root="$(VCPKG_MANIFEST)" \
			--x-install-root="$(VCPKG_INSTALLED)" \
			--triplet=x64-linux \
			--host-triplet=x64-linux

	@echo ">> Configurando build com CMake..."
	@export VCPKG_ROOT="$(VCPKG_ROOT)" && \
		export VCPKG_INSTALLED_DIR="$(VCPKG_INSTALLED)" && \
		mkdir -p "$(BUILD_DIR)/linux-release" && \
		cmake --preset linux-release \
			-DCMAKE_BUILD_TYPE=$(CMAKE_BUILD_TYPE) \
			-DTOGGLE_BIN_FOLDER=ON \
			-DOPTIONS_ENABLE_IPO=OFF \
			-DOTCLIENT_BUILD_TESTS=OFF \
			-DBUILD_STATIC_LIBRARY=ON \
			-DVCPKG_MANIFEST_INSTALL=OFF \
			-DCMAKE_TOOLCHAIN_FILE="$(VCPKG_ROOT)/scripts/buildsystems/vcpkg.cmake" \
			-DVCPKG_INSTALLED_DIR="$(VCPKG_INSTALLED)"

	@echo ">> Compilando otclient..."
	@export VCPKG_ROOT="$(VCPKG_ROOT)" && \
		export VCPKG_INSTALLED_DIR="$(VCPKG_INSTALLED)" && \
		cmake --build --preset linux-release --target otclient -j$$(nproc)

	@echo ">> Empacotando binario + assets..."
	@mkdir -p "$(DIST_DIR)" && \
		(cp "$(BUILD_DIR)/linux-release/bin/silverlakeot" "$(DIST_DIR)/$(BIN_NAME)" 2>/dev/null || cp "$(BUILD_DIR)/linux-release/bin/otclient" "$(DIST_DIR)/$(BIN_NAME)" 2>/dev/null || cp "$(BUILD_DIR)/linux-release/bin/*" "$(DIST_DIR)/" 2>/dev/null || true) && \
		chmod +x "$(DIST_DIR)/$(BIN_NAME)" 2>/dev/null || true && \
		cp -r data mods modules init.lua otclientrc.lua "$(DIST_DIR)/" 2>/dev/null || true && \
		([ -f cacert.pem ] && cp cacert.pem "$(DIST_DIR)/" || true) && \
		([ -f config.ini ] && cp config.ini "$(DIST_DIR)/" || true)

	@echo ">> Build para Ubuntu concluído! Binario em $(DIST_DIR)/$(BIN_NAME)"
