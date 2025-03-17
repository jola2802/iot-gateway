# IoT-Gateway

### Hauptfunktionen

- **Multi-Protokoll-Support**: 
  - Siemens S7 (S7-300, S7-400, S7-1200, S7-1500)
  - OPC UA
  - MQTT

- **Datenverarbeitung und -weiterleitung**:
  - Speicherung in InfluxDB für Zeitreihendaten
  - Integration mit Node-RED für benutzerdefinierte Datenflüsse
  - Bildverarbeitung und -speicherung

- **Benutzerfreundliche Weboberfläche**:
  - Konfiguration von Geräten und Datenpunkten
  - Visualisierung von Daten
  - Verwaltung von Verbindungen

- **Integrierter MQTT-Broker**:
  - Für die interne Kommunikation
  - Unterstützt externe MQTT-Clients

## Systemarchitektur

Das Gateway besteht aus mehreren Komponenten, die in Docker-Containern laufen:

1. **IoT-Gateway**: Die Hauptanwendung, die die Protokolltreiber, Datenverarbeitung und Weboberfläche bereitstellt
2. **Node-RED**: Für benutzerdefinierte Datenflüsse und -verarbeitung
3. **InfluxDB**: Zeitreihendatenbank für die Speicherung von Prozessdaten
4. **NGINX**: Reverse-Proxy für die sichere Kommunikation und Zugriffsverwaltung

## Schnellstart mit Docker Compose

### Voraussetzungen

- Docker und Docker Compose installiert
- Git (optional, für das Klonen des Repositories)

### Installation und Start

1. Repository klonen oder herunterladen:
   ```bash
   git clone https://github.com/jola2802/iot-gateway-neu.git
   cd iot-gateway-neu
   ```

2. Umgebungsvariablen konfigurieren (optional):
   Die Standardkonfiguration in der `.env`-Datei kann bei Bedarf angepasst werden.

3. Gateway starten:
   ```bash
   docker-compose up -d
   ```

   Dieser Befehl startet alle erforderlichen Container im Hintergrund.

4. Auf die Weboberfläche zugreifen:
   - Gateway-UI: https://localhost/
   - Node-RED: https://localhost/nodered/

### Ports (default)
- **443**: HTTPS-Zugriff auf alle Dienste (über NGINX)
- **5000**: MQTT via TCP (without TLS)
- **5001**: MQTT via Websocket (without TLS)
- **5100**: MQTT via Websocket (with TLS)
- **5100**: MQTT via TCP (with TLS) 

### Ports internal (default)
- **8088**: Zugriff auf das Gateway
- **1880**: Zugriff auf Node-RED
- **8086**: Zugriff auf InfluxDB

## Konfiguration

Die Hauptkonfiguration erfolgt über die Weboberfläche. Dort können Sie:

1. Geräte hinzufügen und konfigurieren
2. Datenpunkte definieren
3. Datenweiterleitung einrichten
4. Bildverarbeitung konfigurieren

## Entwicklung

Das Gateway ist in Go geschrieben und verwendet Webtechnologien für die Benutzeroberfläche.


![software-architecture](https://github.com/jola2802/iot-gateway/assets/99344189/6d9614d5-e93f-4640-ad7a-ee7fd8f940f0)
![image](https://github.com/jola2802/iot-gateway/assets/99344189/dc17c774-fe7e-40bd-94d5-4ac419c14b5d)
