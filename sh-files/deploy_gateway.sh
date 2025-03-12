#!/bin/bash

echo "Gib die Netzwerkschnittstelle für die IP-Adresse an:"
interfaces=($(ip -o link show | awk -F': ' '{print $2}'))
for i in "${!interfaces[@]}"; do
    echo "$i: ${interfaces[$i]}"
done

read -p "Wähle die Nummer der Schnittstelle: " iface_index
INTERFACE=${interfaces[$iface_index]}
IP_ADDR=$(ip -o -4 addr show "$INTERFACE" | awk '{print $4}' | cut -d'/' -f1)

if [[ -z "$IP_ADDR" ]]; then
    echo "Fehler: Keine gültige IP-Adresse gefunden!"
    exit 1
fi

read -p "Gib die GitHub-URL des Projekts ein: " GIT_URL
PROJECT_DIR="$HOME/project"

echo "Projekt wird geklont nach $PROJECT_DIR..."
git clone "$GIT_URL" "$PROJECT_DIR"

cd "$PROJECT_DIR" || { echo "Fehler: Projektordner nicht gefunden"; exit 1; }

if [ -f .env ]; then
    echo "Setze IP-Adresse in .env..."
    sed -i "s/^IP_ADDRESS=.*/IP_ADDRESS=$IP_ADDR/" .env
else
    echo "Erstelle neue .env-Datei mit IP-Adresse..."
    echo "IP_ADDRESS=$IP_ADDR" > .env
fi

echo "Docker-Compose Build wird gestartet..."
docker compose build

echo "Deployment abgeschlossen!"
