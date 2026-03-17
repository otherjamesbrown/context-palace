#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

INSTALL_DIR="${INSTALL_DIR:-$HOME/bin}"

echo "Building cxpv..."
go build -o "${INSTALL_DIR}/cxpv" ./cmd/cxpv/
echo "Installed cxpv -> ${INSTALL_DIR}/cxpv"

echo "Building cxp..."
go build -o "${INSTALL_DIR}/cxp" .
echo "Installed cxp -> ${INSTALL_DIR}/cxp"
