## 📋 Schnellstart-Checkliste

- [ ] Docker und Docker Compose installiert
- [ ] Repository heruntergeladen  
- [ ] `.env`-Datei konfiguriert
- [ ] SSL-Zertifikate erstellt (optional)
- [ ] Container gestartet
- [ ] WebUI-Zugriff getestet

---

## 🐳 Docker Installation

---

## 📥 Gateway herunterladen

### Option 1: Mit Git (empfohlen)
```bash
git clone https://github.com/jola2802/iot-gateway-neu.git
cd iot-gateway-neu
```

### Option 2: ZIP-Download
1. Gehen Sie zu: https://github.com/jola2802/iot-gateway-neu
2. Klicken Sie auf "Code" → "Download ZIP"
3. Entpacken Sie das ZIP-Archiv
4. Öffnen Sie Terminal im entpackten Ordner

---

## ⚙️ Konfiguration erstellen

### 1. .env-Datei erstellen

```bash
# .env-Datei aus Template kopieren
cat > .env << 'EOF'
# InfluxDB Konfiguration
INFLUXDB_URL=http://influxdb:8086
INFLUXDB_TOKEN=mein-super-geheimer-token-2024
INFLUXDB_ORG=iot-gateway
INFLUXDB_BUCKET=iot-data
INFLUXDB_ADMIN_USER=admin
INFLUXDB_ADMIN_PASSWORD=sicheres-passwort-123
INFLUXDB_RETENTION=30d
INFLUXDB_HTTP_PORT=8086

# Gateway Konfiguration
WEBUI_HTTP_PORT=8088

# MQTT Konfiguration
MQTT_LISTENER_LOCAL_TCP_ADDRESS=5000
MQTT_LISTENER_LOCAL_WS_ADDRESS=5001
MQTT_LISTENER_PUBLIC_TCP_ADDRESS=5100
MQTT_LISTENER_PUBLIC_WS_ADDRESS=5101

# Node-RED Konfiguration
NODE_RED_HTTP_PORT=1880
NODE_RED_CREDENTIAL_SECRET=sehr-geheime-node-red-schluessel

# NGINX Konfiguration
NGINX_HTTPS_PORT=443
NGINX_TLS_CERT=./server.crt
NGINX_TLS_KEY=./server.key

# Daten-Pfad
DATA_PATH=./data
EOF
```

### 2. Sicherheitseinstellungen anpassen

⚠️ **WICHTIG**: Ändern Sie diese Standardwerte vor dem produktiven Einsatz:

```bash
# Starke Passwörter generieren
INFLUXDB_TOKEN=$(openssl rand -base64 32)
INFLUXDB_ADMIN_PASSWORD=$(openssl rand -base64 16)
NODE_RED_CREDENTIAL_SECRET=$(openssl rand -base64 32)

echo "Neue Tokens generiert:"
echo "INFLUXDB_TOKEN=$INFLUXDB_TOKEN"
echo "INFLUXDB_ADMIN_PASSWORD=$INFLUXDB_ADMIN_PASSWORD"
echo "NODE_RED_CREDENTIAL_SECRET=$NODE_RED_CREDENTIAL_SECRET"
```

---

## 🔐 SSL-Zertifikate erstellen

### Selbstsignierte Zertifikate (für Test/Entwicklung):

```bash
# Zertifikat erstellen
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes \
  -subj "/C=DE/ST=Bayern/L=Ansbach/O=IPDM/OU=IoT-Gateway/CN=localhost"

# Berechtigungen setzen
chmod 600 server.key
chmod 644 server.crt

echo "✅ SSL-Zertifikate erfolgreich erstellt!"
```

---

## 🚀 Gateway starten

### 1. Container-Images bauen und starten

```bash
# Alle Services starten (dauert beim ersten Mal länger)
docker compose up -d

# Fortschritt verfolgen
docker compose logs -f
```

### 2. Startup-Status überprüfen

```bash
# Container-Status anzeigen
docker compose ps
```

### 3. Logs bei Problemen prüfen

```bash
# Alle Logs anzeigen
docker compose logs

# Spezifische Service-Logs
docker compose logs iot-gateway
docker compose logs influxdb
docker compose logs node-red
docker compose logs nginx
```

---

## 🌐 Zugriff testen

### 1. Gateway WebUI
```bash
# Browser öffnen
open https://localhost/
# oder
curl -k https://localhost/

# Standard-Login:
# Benutzername: admin / idpm
# Passwort: password / ansbach
```

### 2. Node-RED Editor
```bash
# Browser öffnen
open https://localhost/nodered/
```

---

## 🔧 Erste Konfiguration

### 1. InfluxDB-Verbindung einrichten

1. **Gateway WebUI öffnen**: https://localhost/
2. **Anmelden** mit admin/password
3. **Data Forwarding** → **InfluxDB** aufrufen
4. **Konfiguration eingeben**:
   ```
   URL: http://influxdb:8086
   Token: [Ihr INFLUXDB_TOKEN aus .env]
   Organization: iot-gateway
   Bucket: iot-data
   ```
5. **"Test Connection"** klicken
6. **"Save Configuration"** klicken

### 2. Erstes Gerät hinzufügen

1. **Device Management** aufrufen
2. **"Add Device"** klicken
3. **Gerätetyp auswählen**:
   - **S7**: Für Siemens PLCs
   - **OPC-UA**: Für OPC-UA Server
   - **MQTT**: Für MQTT-Devices
4. **Verbindungsparameter eingeben**
5. **Datenpunkte konfigurieren**
6. **"Save Device"** klicken

### 3. Live-Daten überprüfen

1. **Devices-Seite** aufrufen
2. **Live Data** Accordion aufklappen
3. **Datenwerte** sollten erscheinen
4. **Historical Data** für Charts aufrufen

---

## 🛡️ Produktive Deployment-Tipps

### Sicherheit:
- [ ] Standard-Passwörter ändern
- [ ] Echte SSL-Zertifikate verwenden
- [ ] VPN für externen Zugriff
- [ ] Regelmäßige Backups

### Performance:
- [ ] Hardware-Ressourcen überwachen
- [ ] InfluxDB-Retention anpassen
- [ ] Log-Rotation einrichten
- [ ] Monitoring implementieren

### Backup:
```bash
# Datenbank-Backup erstellen
docker compose exec influxdb influx backup /tmp/backup
docker compose cp influxdb:/tmp/backup ./backup-$(date +%Y%m%d)

# Konfiguration sichern
cp .env .env.backup
tar -czf config-backup-$(date +%Y%m%d).tar.gz .env iot_gateway.db
```

---

**✅ Installation erfolgreich abgeschlossen!**
