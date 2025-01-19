// wird erstmal nicht benutzt; wenn Konfiguration geändert wurde, so soll der entsprechende Command mitgesendet werden von Web-UI

package logic

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var lastModTime time.Time

func getConfigModTime(configPath string) (time.Time, error) {
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not get file info: %v", err)
	}
	return fileInfo.ModTime(), nil
}

func hasConfigChanged(configPath string) (bool, error) {
	currentModTime, err := getConfigModTime(configPath)
	if err != nil {
		return false, err
	}

	if currentModTime != lastModTime {
		lastModTime = currentModTime
		return true, nil
	}
	return false, nil
}

func WatchConfig(configPath string, interval time.Duration, onChange func()) {
	for {
		changed, err := hasConfigChanged(configPath)
		if err != nil {
			fmt.Printf("Error checking config change: %v", err)
			time.Sleep(interval)
			continue
		}

		if changed {
			fmt.Println("Config file has changed.")
			onChange()
		}

		time.Sleep(interval)
	}
}

func GetConfigPath() (string, error) {
	// Get the current working directoy
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v", err)
		return "", fmt.Errorf("Error getting current directory: %v", err)
	}
	// Build the configPath variable
	configPath := filepath.Join(dir + "/config.json")
	return configPath, nil
}
