package dataforwarding

import (
	"database/sql"
	"log"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

// Weiterleitung von Daten von einem internen MQTT-Topic zu einem öffentlichen Topic
func ForwardToPublicTopic(db *sql.DB, topic, payload string) error {
	// Schritt 1: Hole die Zugangsdaten für den MQTT-Broker aus der Tabelle auth
	var broker, username, password string
	query := `
		SELECT bs.address AS broker, a.username, a.password
		FROM broker_settings bs
		JOIN auth a ON a.username = 'webui-admin'
		LIMIT 1
	`
	err := db.QueryRow(query).Scan(&broker, &username, &password)
	if err != nil {
		log.Printf("Error loading MQTT config from DB: %v", err)
		return err
	}

	// Schritt 2: MQTT-Client konfigurieren
	opts := MQTT.NewClientOptions().
		AddBroker(broker).
		SetUsername(username).
		SetPassword(password).
		SetPingTimeout(10 * time.Second).
		SetAutoReconnect(true)

	client := MQTT.NewClient(opts)

	// Schritt 3: Mit dem MQTT-Broker verbinden
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("Error connecting to MQTT broker: %v", token.Error())
		return token.Error()
	}
	defer client.Disconnect(250)
	// Definiere das öffentliche Topic (dies könnte dynamisch oder konfigurierbar sein)
	publicTopic := "public/" + topic

	// Erstelle die Nachricht
	token := client.Publish(publicTopic, 0, false, payload)
	token.Wait()

	if token.Error() != nil {
		log.Printf("Error forwarding message to public topic: %v", token.Error())
		return token.Error()
	}

	// log.Printf("Forwarded message to public topic: %s, Payload: %s", publicTopic, payload)
	return nil
}

// Weiterleitung von Daten zu einem externen MQTT-Broker
func ForwardToExternalBroker(route DataRoute, topic, payload string) error {
	opts := MQTT.NewClientOptions().
		// AddBroker(route.ExternalBroker).
		// SetUsername(route.BrokerUsername).
		// SetPassword(route.BrokerPassword).
		SetAutoReconnect(true)

	client := MQTT.NewClient(opts)

	// Verbindung zu externem Broker herstellen
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("Error connecting to external MQTT broker: %v", token.Error())
		return token.Error()
	}
	defer client.Disconnect(250)

	// Daten an das entsprechende Topic senden
	token := client.Publish(topic, 0, false, payload)
	token.Wait()

	if token.Error() != nil {
		log.Printf("Error forwarding message to external MQTT broker: %v", token.Error())
		return token.Error()
	}

	log.Printf("Forwarded message to external MQTT broker: %s, Payload: %s", topic, payload)
	return nil
}
