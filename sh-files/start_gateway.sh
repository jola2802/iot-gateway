#!/bin/bash

PROJECT_DIR="$HOME/project"

if [ -d "$PROJECT_DIR" ]; then
    cd "$PROJECT_DIR"
    echo "Starte Gateway-Container..."
    docker compose up -d
    echo "Container gestartet."
else
    echo "Projektverzeichnis nicht gefunden!"
fi
