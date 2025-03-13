#!/bin/bash

PROJECT_DIR="$HOME/project"

if [ -d "$PROJECT_DIR" ]; then
    cd "$PROJECT_DIR"
    echo "Stoppe Gateway-Container..."
    docker compose down
    echo "Container gestoppt."
else
    echo "Projektverzeichnis nicht gefunden!"
fi

read -p "Drücke Enter"