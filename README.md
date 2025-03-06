# IoT-Gateway

Ein leistungsstarkes und flexibles IoT-Gateway zur Datenerfassung, -verarbeitung und -weiterleitung von industriellen Anlagen und IoT-Geräten.

## Überblick

Dieses IoT-Gateway dient als zentrale Schnittstelle zwischen verschiedenen industriellen Systemen und IoT-Geräten. Es ermöglicht die Erfassung, Verarbeitung und Weiterleitung von Daten aus unterschiedlichen Quellen und unterstützt mehrere Industrieprotokolle.

### Hauptfunktionen

- **Multi-Protokoll-Unterstützung**: 
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

![software-architecture](https://github.com/jola2802/iot-gateway/assets/99344189/6d9614d5-e93f-4640-ad7a-ee7fd8f940f0)

## Schnellstart mit Docker Compose

### Voraussetzungen

- Docker und Docker Compose installiert
- Git (optional, für das Klonen des Repositories)

### Installation und Start

1. Repository klonen oder herunterladen:
   ```bash
   git clone https://github.com/yourusername/iot-gateway.git
   cd iot-gateway
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
   - Node-RED: https://localhost/node-red/
   - InfluxDB: https://localhost/influxdb/

### Ports

- **5000-5003**: Reserviert für interne Dienste
- **8088**: HTTP-Zugriff auf das Gateway
- **443**: HTTPS-Zugriff auf alle Dienste (über NGINX)

## Konfiguration

Die Hauptkonfiguration erfolgt über die Weboberfläche. Dort können Sie:

1. Geräte hinzufügen und konfigurieren
2. Datenpunkte definieren
3. Datenweiterleitung einrichten
4. Bildverarbeitung konfigurieren

## Entwicklung

Das Gateway ist in Go geschrieben und verwendet moderne Webtechnologien für die Benutzeroberfläche. Die modulare Architektur ermöglicht einfache Erweiterungen und Anpassungen.

![image](https://github.com/jola2802/iot-gateway/assets/99344189/dc17c774-fe7e-40bd-94d5-4ac419c14b5d)