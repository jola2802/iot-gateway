## 📋 Schnellstart-Checkliste

- [ ] Docker und Docker Compose installiert
- [ ] Repository heruntergeladen  
- [ ] `.env`-Datei konfiguriert
- [ ] Container gestartet
- [ ] WebUI-Zugriff getestet

---

## 📥 Gateway herunterladen

### Option 1: Mit Git
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

### 1. .env-Datei umbennen und bearbeiten

# InfluxDB Konfiguration
INFLUXDB_URL=http://influxdb:8086
INFLUXDB_TOKEN=secret
INFLUXDB_ORG=iot-gateway
INFLUXDB_BUCKET=iot-data
INFLUXDB_ADMIN_USER=admin
INFLUXDB_ADMIN_PASSWORD=password
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
NGINX_HTTPS_PORT=8088
NGINX_TLS_CERT=./server.crt
NGINX_TLS_KEY=./server.key

# Daten-Pfad
DATA_PATH=./data
EOF
```


## 🚀 Gateway starten

### 1. Container-Images bauen und starten

```bash
# Alle Services starten (dauert beim ersten Mal länger, ca. 5-10 min)
docker compose up -d
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
open https://localhost:8088

# Standard-Login Gateway:
# Benutzername: admin
# Passwort: password
```

### 2. Node-RED Editor
```bash
# Browser öffnen
open https://localhost:1880
# Standard-Login Node-RED:
# Benutzername: idpm
# Passwort: ansbach
```

### 3. InfluxDB
```bash
# Browser öffnen
open https://localhost:8086
# Standard-Login InfluxDB:
# Benutzername: admin
# Passwort: password
```

---

**✅ Installation erfolgreich abgeschlossen!**
