package logic

import (
	"time"
)

// Letzte bekannte Werte, um Änderungen zu verfolgen
var lastKnownValues = make(map[string]interface{})
var lastSentTimes = make(map[string]time.Time)

// ShouldSendData prüft, ob die Daten gesendet werden sollen
func ShouldSendData(nodeID string, newValue interface{}, logik string, acquisitionTime int) bool {
	currentTime := time.Now()

	switch logik {
	case "zyklisch":
		if lastTime, ok := lastSentTimes[nodeID]; ok {
			if currentTime.Sub(lastTime) >= time.Duration(acquisitionTime)*time.Second {
				lastSentTimes[nodeID] = currentTime
				return true
			}
		} else {
			lastSentTimes[nodeID] = currentTime
			return true
		}
	case "beiAenderung":
		if lastValue, ok := lastKnownValues[nodeID]; ok {
			if lastValue != newValue {
				lastKnownValues[nodeID] = newValue
				return true
			}
		} else {
			lastKnownValues[nodeID] = newValue
			return true
		}
	}
	return false
}
