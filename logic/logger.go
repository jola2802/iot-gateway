package logic

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// Globale Variablen für das In-Memory-Logging-System
var log = logrus.New()    // Eigene Logger-Instanz
var logMutex sync.Mutex   // Mutex für Thread-Safety
var inMemoryLogs []string // Speicher für Log-Einträge
var maxLogEntries = 300   // Maximale Anzahl von Log-Einträgen im Speicher

// init wird automatisch beim Import des Pakets aufgerufen
// und konfiguriert sowohl den globalen logrus.Logger als auch
// unsere eigene log-Instanz, damit alle Logs im Speicher erfasst werden.
func init() {
	// Standardmäßiges Logging-Format (JSON)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	// Log-Level für globalen und lokalen Logger setzen
	logrus.SetLevel(logrus.InfoLevel)
	log.SetLevel(logrus.InfoLevel)

	// Globalen Logger konfigurieren (erfasst alle logrus.Info, logrus.Error, etc. im Code)
	logrus.AddHook(&memoryHook{})

	// Lokalen Logger konfigurieren (erfasst alle log.Info, log.Error, etc.)
	log.AddHook(&memoryHook{})

	// Speicher für Logs initialisieren
	inMemoryLogs = make([]string, 0, maxLogEntries)
}

// GatewayLogs initialisiert das Logging-System.
// Diese Funktion sollte früh im Programmablauf aufgerufen werden.
// Sie loggt nur eine Startnachricht, die eigentliche Konfiguration
// passiert bereits in der init()-Funktion.
func GatewayLogs() {
	// log.Info("Logging-System initialisiert")
	// logrus.Info("Globales Logging aktiviert - alle logrus-Aufrufe werden erfasst")
}

// GetLogger gibt die Logger-Instanz zurück, die für
// direkte Logger-Aufrufe verwendet werden kann.
// Alternativ kann auch direkt der globale logrus-Logger verwendet werden.
func GetLogger() *logrus.Logger {
	return log
}

// GetLogs gibt alle gespeicherten Logs zurück.
// Diese Funktion wird vom webui-Paket aufgerufen, um
// die Logs auf der Settings-Seite anzuzeigen.
func GetLogs() []string {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Kopie erstellen, um Race-Conditions zu vermeiden
	logsCopy := make([]string, len(inMemoryLogs))
	copy(logsCopy, inMemoryLogs)

	return logsCopy
}

// ClearLogs löscht alle gespeicherten Logs
func ClearLogs() {
	logMutex.Lock()
	defer logMutex.Unlock()

	inMemoryLogs = make([]string, 0, maxLogEntries)
}

// addLogEntry fügt einen Log-Eintrag zum Speicher hinzu
// und implementiert einen Ring-Buffer, der alte Einträge
// verwirft, wenn der Speicher voll ist.
func addLogEntry(entry string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	// Wenn der Buffer voll ist, entferne den ältesten Eintrag
	if len(inMemoryLogs) >= maxLogEntries {
		inMemoryLogs = inMemoryLogs[1:]
	}

	// Füge den neuen Eintrag hinzu
	inMemoryLogs = append(inMemoryLogs, entry)
}

// memoryHook ist ein Hook für Logrus, der Logs im Speicher hält.
// Dieser Hook wird sowohl für den globalen Logger als auch für
// unsere lokale Logger-Instanz registriert.
type memoryHook struct{}

// Fire wird aufgerufen, wenn ein Log-Eintrag erzeugt wird.
// Diese Methode konvertiert den Log-Eintrag in einen String
// und fügt ihn zum Speicher hinzu.
func (hook *memoryHook) Fire(entry *logrus.Entry) error {
	// Konvertiere den Log-Eintrag in einen String
	line, err := entry.String()
	if err != nil {
		return err
	}

	// Füge den Eintrag zum Speicher hinzu
	addLogEntry(line)

	return nil
}

// Levels gibt die Log-Level zurück, für die der Hook aktiviert ist.
// Wir erfassen alle Level, von Debug bis Fatal.
func (hook *memoryHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
