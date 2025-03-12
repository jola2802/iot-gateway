#!/bin/bash

echo "Stoppe alle laufenden Container..."
docker stop $(docker ps -aq) 2>/dev/null

echo "Lösche alle Container..."
docker rm $(docker ps -aq) 2>/dev/null

echo "Lösche alle Docker-Images..."
docker rmi -f $(docker images -q) 2>/dev/null

echo "Lösche alle ungenutzten Docker-Volumes..."
docker volume prune -f

echo "Docker-Bereinigung abgeschlossen!"

read -p "Drücken Sie Enter, um die Bereinigung abzuschließen..."