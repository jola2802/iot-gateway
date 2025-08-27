# Node-RED File Access zu C:/Dokumente

## 📁 Konfiguration

### Docker-Compose Volumes:
```yaml
volumes:
  - ${DATA_PATH}:/data/shared/           # Bestehender Gateway-Data Pfad
  - C:/Dokumente:/data/images         # Neuer lokaler Dokumente-Zugriff
  - ./node-red-data:/data               # Node-RED Konfiguration
```

### Node-RED Settings:
```javascript
fileWorkingDirectory: "/data/images"  // Standard-Arbeitsverzeichnis für File-Nodes
```

## 🔧 Verwendung in Node-RED

### 1. **Absolute Pfade:**
```javascript
// In Function-Nodes:
const filePath = "/data/images/meine-datei.txt";
fs.writeFileSync(filePath, "Inhalt");
```

### 2. **Relative Pfade:**
```javascript
// Relativ zum fileWorkingDirectory (/data/documents):
const filePath = "unterordner/datei.txt";  // → C:/Dokumente/unterordner/datei.txt
fs.writeFileSync(filePath, "Inhalt");
```

### 3. **File-In/File-Out Nodes:**
- **File-Out Node**: Pfad eingeben als `/data/documents/dateiname.txt` oder relativ `dateiname.txt`
- **File-In Node**: Pfad eingeben als `/data/documents/dateiname.txt` oder relativ `dateiname.txt`

## 📂 Verfügbare Pfade in Node-RED:

| Node-RED Pfad | Windows Pfad | Beschreibung |
|---------------|--------------|--------------|
| `/data/documents/` | `C:/Dokumente/` | **Neuer lokaler Zugriff** |
| `/data/shared/` | `${DATA_PATH}` | Gateway-Data (Images, etc.) |
| `/data/` | `./node-red-data/` | Node-RED Konfiguration |

## 🚀 Beispiel Function-Node:

```javascript
// Datei in C:/Dokumente speichern
const timestamp = new Date().toISOString();
const fileName = `sensor-data-${timestamp.replace(/[:.]/g, '-')}.json`;
const filePath = `/data/documents/${fileName}`;

const data = {
    timestamp: timestamp,
    temperature: msg.payload.temperature,
    humidity: msg.payload.humidity
};

try {
    fs.writeFileSync(filePath, JSON.stringify(data, null, 2));
    node.status({fill: "green", shape: "dot", text: `Saved: ${fileName}`});
    msg.filePath = filePath;
    return msg;
} catch (error) {
    node.error(`File write error: ${error.message}`);
    node.status({fill: "red", shape: "ring", text: "Save failed"});
    return null;
}
```

## ⚠️ Wichtige Hinweise:

1. **Windows-Pfade**: Der Container kann nicht direkt auf Windows-Laufwerke zugreifen, nur über Docker-Volumes
2. **Berechtigungen**: Docker muss Schreibrechte auf `C:/Dokumente` haben
3. **Pfad-Separator**: Verwende `/` statt `\` in Node-RED (Linux-Container)
4. **Ordner-Erstellung**: Falls Unterordner nicht existieren, verwende `fs.mkdirSync(pfad, {recursive: true})`

## 🔄 Nach Änderungen:

```bash
# Docker-Container neu starten:
docker-compose down
docker-compose up -d --build
```
