#!/bin/bash

LOGFILE="$HOME/Desktop/setup.log"
echo "==== Raspberry Pi Setup gestartet am $(date) ====" | tee -a $LOGFILE

echo "1. System aktualisieren..." | tee -a $LOGFILE
sudo apt update && sudo apt upgrade -y 2>&1 | tee -a $LOGFILE

echo "2. Erforderliche Pakete installieren..." | tee -a $LOGFILE
sudo apt install -y ca-certificates curl gnupg 2>&1 | tee -a $LOGFILE

echo "3. Docker Repository hinzufügen..." | tee -a $LOGFILE
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo tee /etc/apt/keyrings/docker.asc > /dev/null
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

echo "4. Docker installieren..." | tee -a $LOGFILE
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin 2>&1 | tee -a $LOGFILE

echo "5. Docker-Dienst aktivieren..." | tee -a $LOGFILE
sudo systemctl enable --now docker 2>&1 | tee -a $LOGFILE

echo "6. Prüfen, ob Docker läuft..." | tee -a $LOGFILE
sudo docker run hello-world 2>&1 | tee -a $LOGFILE

echo "7. Benutzer zur Docker-Gruppe hinzufügen..." | tee -a $LOGFILE
sudo usermod -aG docker $USER
newgrp docker

echo "==== Setup abgeschlossen am $(date) ====" | tee -a $LOGFILE
