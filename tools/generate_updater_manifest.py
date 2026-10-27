#!/usr/bin/env python3
"""
generate_updater_manifest.py

Gera o arquivo updater.json para o auto-updater do OTClient.
Calcula o CRC32 (em hexadecimal minúsculo) de cada arquivo relativo à raiz do client,
compatível com Crypt::crc32 do C++.
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


def scan_client_files(client_root: str):
    """Varre os arquivos do client e retorna um dicionario { 'caminho/relativo': 'crc32' }."""
    files_map = {}

    # 1. Arquivos da raiz explicitos (ex: init.lua)
    for root_file in DEFAULT_ROOT_FILES:
        full_path = os.path.join(client_root, root_file)
        if os.path.isfile(full_path):
            files_map[root_file] = calculate_crc32(full_path)

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
                rel_path = os.path.relpath(full_path, client_root).replace("\\", "/")
                files_map[rel_path] = calculate_crc32(full_path)

    return files_map


def main():
    parser = argparse.ArgumentParser(description="Gerador de Manifesto updater.json para OTClient")
    parser.add_argument(
        "--client-dir",
        default=".",
        help="Diretorio raiz do client (padrao: diretorio atual)",
    )
    parser.add_argument(
        "--url",
        default="https://downloads.silverlakeot.com/client/",
        help="URL base onde os arquivos atualizados serao baixados",
    )
    parser.add_argument(
        "--binary",
        default=None,
        help="Nome ou caminho do executavel do client (ex: SilverLake.exe)",
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

    print(f"Varrendo arquivos em: {client_root}")
    files_checksums = scan_client_files(client_root)
    print(f"Total de arquivos mapeados: {len(files_checksums)}")

    url = args.url if args.url.endswith("/") else args.url + "/"

    manifest = {
        "url": url,
        "files": files_checksums,
    }

    if args.binary:
        binary_path = args.binary
        if not os.path.isabs(binary_path):
            binary_path = os.path.join(client_root, binary_path)

        binary_filename = os.path.basename(args.binary)
        if os.path.isfile(binary_path):
            binary_crc = calculate_crc32(binary_path)
            manifest["binary"] = {
                "file": binary_filename,
                "checksum": binary_crc,
            }
            print(f"Binario mapeado: {binary_filename} (CRC32: {binary_crc})")
        else:
            print(f"Aviso: Binario '{binary_path}' nao encontrado. O bloco 'binary' nao contera checksum.")
            manifest["binary"] = {
                "file": binary_filename,
                "checksum": "",
            }

    output_path = os.path.abspath(args.output)
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2)

    print(f"Manifesto gerado com sucesso em: {output_path}")


if __name__ == "__main__":
    main()
