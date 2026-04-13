#!/usr/bin/env bash

set -e

BINARY_NAME="commitgen"
INSTALL_DIR="/usr/local/bin"

echo "Building $BINARY_NAME..."
go build -o "$BINARY_NAME" .

echo "Installing to $INSTALL_DIR/$BINARY_NAME..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
else
    sudo mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
fi

echo "Done. Run '$BINARY_NAME' to get started."
