package features

import (
	"bufio"
	"os/exec"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/sirupsen/logrus"
)

// Variable, um den gestarteten Node-RED Prozess zu speichern
var nodeRedCmd *exec.Cmd

func StartNodeRed() (string, error) {
	// 1. Prüfen, ob "node-red" im Pfad vorhanden ist.
	_, err := exec.LookPath("node-red")
	if err != nil {
		logrus.Error("Node-RED ist nicht installiert oder nicht im Systempfad verfügbar.")
		return "", err
	}

	// 2. Prüfen, ob Node-RED bereits läuft.
	if isNodeRedRunning() {
		logrus.Info("Node-RED läuft bereits.")
		return "", nil
	}

	// 3. Node-RED starten.
	nodeRedCmd = exec.Command("node-red")

	// 4. Stdout-Pipe abfragen.
	stdout, err := nodeRedCmd.StdoutPipe()
	if err != nil {
		logrus.Errorf("Fehler beim Abrufen des Stdout-Pipes: %v", err)
		return "", err
	}

	// 5. Node-RED-Prozess starten.
	if err := nodeRedCmd.Start(); err != nil {
		logrus.Errorf("Fehler beim Starten von Node-RED: %v", err)
		nodeRedCmd = nil
		return "", err
	}

	// 6. Kanal für die URL erstellen
	urlChan := make(chan string)

	// 7. Goroutine, um stdout zu parsen
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// Nach einer URL (http:// oder https://) suchen
			if strings.Contains(line, "http://") || strings.Contains(line, "https://") {
				urlChan <- extractAndLogURL(line)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			logrus.Errorf("Fehler beim Lesen der Node-RED-Stdout-Ausgabe: %v", err)
		}
	}()

	// logrus.Info("Node-RED wurde erfolgreich gestartet.")

	// 9. Auf die URL warten und zurückgeben
	select {
	case url := <-urlChan:
		return url, nil
	case <-time.After(10 * time.Second):
		return "", nil
	}
}

// Funktion zum Stoppen von Node-RED, wenn es von StartNodeRed gestartet wurde
func StopNodeRed() {
	if nodeRedCmd == nil {
		logrus.Info("Node-RED wurde nicht von diesem Programm gestartet oder läuft nicht.")
		return
	}

	// Prozess beenden
	if err := nodeRedCmd.Process.Kill(); err != nil {
		logrus.Errorf("Fehler beim Stoppen von Node-RED: %v", err)
		return
	}

	logrus.Info("Node-RED wurde erfolgreich gestoppt.")
	nodeRedCmd = nil
}

// Hilfsfunktion, um zu prüfen, ob Node-RED bereits läuft
func isNodeRedRunning() bool {
	processes, err := process.Processes()
	if err != nil {
		logrus.Errorf("Fehler beim Abrufen der Prozessliste: %v", err)
		return false
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if name == "node-red" || name == "node-red.exe" {
			return true
		}
	}

	return false
}

// Hilfsfunktion zum Extrahieren und Loggen von URL + Port
func extractAndLogURL(line string) string {
	// Suchen, wo http:// bzw. https:// beginnt
	idxHttp := strings.Index(line, "http://")
	idxHttps := strings.Index(line, "https://")

	var startIndex int
	switch {
	case idxHttp != -1:
		startIndex = idxHttp
	case idxHttps != -1:
		startIndex = idxHttps
	default:
		// Keine URL gefunden
		return ""
	}

	// URL-Teil aus der Zeile herausschneiden
	urlPortion := line[startIndex:] // ab http:// bzw. https:// bis Zeilenende
	// optional am Leerzeichen/Tab bremsen
	endIdx := strings.IndexAny(urlPortion, " \t")
	if endIdx != -1 {
		urlPortion = urlPortion[:endIdx]
	}

	// Port herausziehen
	portPart := strings.Split(urlPortion, ":")
	if len(portPart) > 2 {
		// Beispiel: "http://127.0.0.1:1880/" --> portPart[0] = "http" portPart[1] = "//127.0.0.1" portPart[2] = "1880/"

		port := strings.Split(portPart[2], "/")[0]
		logrus.Infof("Node-RED läuft auf Port: %s (URL: %s)", port, urlPortion)
	} else {
		// Falls kein Port-Teil vorhanden ist (z. B. http://localhost ohne Port)
		logrus.Infof("Node-RED URL gefunden: %s (kein Port angegeben)", urlPortion)
	}

	return urlPortion
}
