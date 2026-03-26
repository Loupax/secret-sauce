#!/usr/bin/env bash
set -euo pipefail

BINARY="sauce"
INSTALL_DIR="$(go env GOPATH)/bin"

echo "Building ${BINARY}..."
go build -o "${BINARY}" .

echo "Installing to ${INSTALL_DIR}/${BINARY}..."
mkdir -p "${INSTALL_DIR}"
mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "Done. Make sure ${INSTALL_DIR} is on your PATH."
