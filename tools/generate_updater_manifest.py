#!/usr/bin/env python3
"""
generate_updater_manifest.py

Gera o arquivo updater.json para o auto-updater do OTClient.
Calcula o CRC32 (em hexadecimal minúsculo) de cada arquivo relativo à raiz do client,
compatível com Crypt::crc32 do C++.
Os caminhos dos arquivos e binários são prefixados com '/' para compatibilidade com
g_resources.filesChecksums() do PhysFS, e a URL base é mantida sem barra final.
Suporta múltiplos binários indexados por SO (windows, linux, mac).
"""

import argparse
import json
import os
import sys
import zlib

DEFAULT_IGNORED_DIRS = {
    ".git",
    ".github",
    ".gemini",
    ".vscode",
    ".idea",
    "build",
    "cmake",
    "src",
    "tests",
    "tools",
    "vcpkg",
    "vcpkg_installed",
    "android",
    "browser",
}

DEFAULT_IGNORED_EXTENSIONS = {
    ".cpp",
    ".hpp",
    ".h",
    ".c",
    ".rc",
    ".patch",
    ".bak",
    ".log",
    ".tmp",
    ".md",
    ".gitignore",
    ".gitattributes",
    ".editorconfig",
    ".toml",
}

DEFAULT_SCAN_DIRS = ["data", "modules"]
DEFAULT_ROOT_FILES = ["init.lua"]


def calculate_crc32(filepath: str) -> str:
    """Calcula o checksum CRC32 em hex minusculo sem zeros a esquerda (como std::to_chars em base 16)."""
    crc = 0
    with open(filepath, "rb") as f:
        while chunk := f.read(65536):
            crc = zlib.crc32(chunk, crc)
    return hex(crc & 0xFFFFFFFF)[2:].lower()


def format_manifest_path(rel_path: str) -> str:
    """Normaliza o caminho com barras comuns e garante a barra '/' no inicio para PhysFS."""
    normalized = rel_path.replace("\\", "/")
    if not normalized.startswith("/"):
        normalized = "/" + normalized
    return normalized


def scan_client_files(client_root: str):
    """Varre os arquivos do client e retorna um dicionario { '/caminho/relativo': 'crc32' }."""
    files_map = {}

    # 1. Arquivos da raiz explicitos (ex: init.lua)
    for root_file in DEFAULT_ROOT_FILES:
        full_path = os.path.join(client_root, root_file)
        if os.path.isfile(full_path):
            files_map[format_manifest_path(root_file)] = calculate_crc32(full_path)

    # 2. Diretorios de assets e scripts
    for folder in DEFAULT_SCAN_DIRS:
        folder_path = os.path.join(client_root, folder)
        if not os.path.isdir(folder_path):
            continue

        for root, dirs, files in os.walk(folder_path):
            dirs[:] = [d for d in dirs if d not in DEFAULT_IGNORED_DIRS]

            for file in files:
                _, ext = os.path.splitext(file)
                if ext.lower() in DEFAULT_IGNORED_EXTENSIONS:
                    continue

                full_path = os.path.join(root, file)
                rel_path = os.path.relpath(full_path, client_root)
                files_map[format_manifest_path(rel_path)] = calculate_crc32(full_path)

    return files_map


def resolve_binary(client_root: str, binary_path: str):
    """Calcula o checksum do binario e retorna dict {'file': '/...', 'checksum': '...'}."""
    if not os.path.isabs(binary_path):
        binary_path = os.path.join(client_root, binary_path)

    filename = format_manifest_path(os.path.basename(binary_path))
    if os.path.isfile(binary_path):
        checksum = calculate_crc32(binary_path)
        print(f"Binario mapeado: {filename} (CRC32: {checksum})")
        return {"file": filename, "checksum": checksum}
    else:
        print(f"Aviso: Binario '{binary_path}' nao encontrado em disco.")
        return {"file": filename, "checksum": ""}


def main():
    parser = argparse.ArgumentParser(description="Gerador de Manifesto updater.json para OTClient")
    parser.add_argument(
        "--client-dir",
        default=".",
        help="Diretorio raiz do client (padrao: diretorio atual)",
    )
    parser.add_argument(
        "--url",
        default="https://downloads.silverlakeot.com/client",
        help="URL base onde os arquivos atualizados serao baixados (sem barra no final)",
    )
    parser.add_argument(
        "--binary",
        default=None,
        help="Binario unico generico (ex: silverlakeot.exe)",
    )
    parser.add_argument(
        "--binary-windows",
        default=None,
        help="Executavel Windows (ex: silverlakeot.exe ou dist/silverlakeot.exe)",
    )
    parser.add_argument(
        "--binary-linux",
        default=None,
        help="Executavel Linux (ex: silverlakeot ou dist/silverlakeot)",
    )
    parser.add_argument(
        "--output",
        default="updater.json",
        help="Arquivo JSON de destino (padrao: updater.json)",
    )

    args = parser.parse_args()
    client_root = os.path.abspath(args.client_dir)

    if not os.path.exists(client_root):
        print(f"Erro: Diretorio do client nao encontrado: {client_root}", file=sys.stderr)
        sys.exit(1)

    output_path = os.path.abspath(args.output)
    existing_binaries = {}
    existing_binary = None

    if os.path.isfile(output_path):
        try:
            with open(output_path, "r", encoding="utf-8") as f:
                old_data = json.load(f)
                if isinstance(old_data.get("binaries"), dict):
                    existing_binaries = old_data["binaries"]
                if isinstance(old_data.get("binary"), dict):
                    existing_binary = old_data["binary"]
        except Exception as e:
            print(f"Aviso ao carregar manifesto anterior: {e}")

    # Garante que os caminhos de binarios existentes tambem usem '/' no inicio
    if existing_binary and isinstance(existing_binary, dict) and "file" in existing_binary:
        existing_binary["file"] = format_manifest_path(existing_binary["file"])
    for k, v in existing_binaries.items():
        if isinstance(v, dict) and "file" in v:
            v["file"] = format_manifest_path(v["file"])

    print(f"Varrendo arquivos em: {client_root}")
    files_checksums = scan_client_files(client_root)
    print(f"Total de arquivos mapeados: {len(files_checksums)}")

    # URL base sempre sem barra no final para evitar URL dupla com os arquivos que comecam com '/'
    url = args.url.rstrip("/")

    manifest = {
        "url": url,
        "files": files_checksums,
    }

    # Binarios por plataforma
    new_binaries = {}
    if args.binary_windows:
        new_binaries["windows"] = resolve_binary(client_root, args.binary_windows)
    if args.binary_linux:
        new_binaries["linux"] = resolve_binary(client_root, args.binary_linux)

    # Mescla entradas de multiplas plataformas preservando o que nao foi sobrescrito
    binaries = {**existing_binaries, **new_binaries}

    if binaries:
        manifest["binaries"] = binaries
        if "windows" in binaries and not args.binary:
            manifest["binary"] = binaries["windows"]
        elif existing_binary and not args.binary:
            manifest["binary"] = existing_binary

    if args.binary and "binary" not in manifest:
        manifest["binary"] = resolve_binary(client_root, args.binary)

    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2)

    print(f"Manifesto gerado com sucesso em: {output_path}")


if __name__ == "__main__":
    main()
