package logic

import (
	"database/sql"
	"os"

	_ "github.com/glebarez/go-sqlite"
	"github.com/sirupsen/logrus"
)

// SQL-Queries als Konstanten definieren
const (
	createAuthTable = `
		CREATE TABLE IF NOT EXISTS auth (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			allow BOOLEAN NOT NULL
		);
	`

	createACLTable = `
		CREATE TABLE IF NOT EXISTS acl (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			topic TEXT NOT NULL,
			permission INTEGER NOT NULL,
			FOREIGN KEY(username) REFERENCES auth(username)
		);
	`

	createUsersTable = `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			name TEXT,
			address TEXT,
			company TEXT,
			email TEXT
		);
	`

	createBrokerSettingsTable = `
		CREATE TABLE IF NOT EXISTS broker_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			address TEXT NOT NULL,
			username TEXT,
			password TEXT
		);
	`

	createSettingsTable = `
		CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY,
			docker_ip TEXT,
			use_custom_services BOOLEAN DEFAULT FALSE,
			nodered_url TEXT,
			influxdb_url TEXT,
			use_external_broker BOOLEAN DEFAULT FALSE,
			broker_url TEXT,
			broker_port TEXT,
			broker_username TEXT,
			broker_password TEXT
		);
	`

	createDevicesTable = `
		CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type VARCHAR(50) NOT NULL,
			name VARCHAR(100) NOT NULL,
			address TEXT NOT NULL,
			acquisition_time INT NOT NULL,
			status TEXT NOT NULL,
			rack INT,					 -- Optional
			slot INT,				     -- Optional
			security_mode TEXT,          -- Optional
			security_policy TEXT,        -- Optional
			certificate TEXT,            -- Optional für Zertifikat-basierte Authentifizierung
			key TEXT,                    -- Optional für Zertifikat-basierte Authentifizierung
			username TEXT,               -- Optional für Username-basierte Authentifizierung
			password TEXT                -- Optional für Passwort-basierte Authentifizierung
		);
	`

	createS7DatapointsTable = `
		CREATE TABLE IF NOT EXISTS s7_datapoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INT NOT NULL,
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL,
			datatype VARCHAR(100) NOT NULL,
			address VARCHAR(20) NOT NULL
		);
	`

	createOPCUADatanodesTable = `
		CREATE TABLE IF NOT EXISTS opcua_datanodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INT NOT NULL,
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL,
			node_identifier VARCHAR(100) NOT NULL
		);
	`

	createMQTTDatapointsTable = `
		CREATE TABLE IF NOT EXISTS opcua_datanodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INT REFERENCES devices(id),
			datapointId VARCHAR(10) NOT NULL,
			name VARCHAR(100) NOT NULL
		);
	`

	createdataRoutesTable = `
		CREATE TABLE IF NOT EXISTS data_routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			destination_type TEXT NOT NULL,
			data_format TEXT NOT NULL,
			interval INTEGER NOT NULL,
			devices TEXT NOT NULL,
			destination_url TEXT,
			headers TEXT, -- Verwende TEXT für JSON-Daten
			file_path TEXT,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`

	createdeviceDataTable = `
		CREATE TABLE IF NOT EXISTS device_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_name TEXT NOT NULL,
			deviceId TEXT,
			datapoint TEXT NOT NULL,
			datapointId TEXT,
			value TEXT NOT NULL, -- Verwende TEXT für JSON-Daten
			timestamp TEXT NOT NULL
		);
	`

	createImageProcessTable = `
		CREATE TABLE IF NOT EXISTS img_process (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device TEXT NOT NULL,
			m_parent_node TEXT NOT NULL,
			m_image_node TEXT NOT NULL,
			method_arguments TEXT NOT NULL,
			image_node TEXT NOT NULL,
			captured_node TEXT NOT NULL,
			timeout TEXT NOT NULL,
			capture_mode TEXT NOT NULL,
			trigger TEXT NOT NULL,
			complete_node TEXT NOT NULL,
			rest_uri TEXT NOT NULL,
			headers TEXT DEFAULT NULL,
			status TEXT DEFAULT NULL,
			status_data TEXT DEFAULT NULL
		);
	`

	createInfluxDBTable = `
		CREATE TABLE IF NOT EXISTS influxdb (
			url TEXT NOT NULL,
			token TEXT NOT NULL,
			org TEXT NOT NULL,
			bucket TEXT NOT NULL
		);
	`

	createImagesTable = `
		CREATE TABLE IF NOT EXISTS images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device TEXT NOT NULL,
			image TEXT NOT NULL,
			timestamp TEXT NOT NULL
		);
	`
)

// InitDB initialisiert die SQLite-Datenbank mit einem übergebenen Pfad
func InitDB(dbPath string) (*sql.DB, error) {
	// Überprüfen, ob die Datenbankdatei existiert
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Datenbankdatei erstellen
		file, err := os.Create(dbPath)
		if err != nil {
			return nil, err
		}
		file.Close()
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// SQL-Tabellen in einer Liste speichern
	tables := []string{
		createAuthTable,
		createACLTable,
		createUsersTable,
		createBrokerSettingsTable,
		createSettingsTable,
		createDevicesTable,
		createS7DatapointsTable,
		createOPCUADatanodesTable,
		createMQTTDatapointsTable,
		createdataRoutesTable,
		createdeviceDataTable,
		createImageProcessTable,
		createInfluxDBTable,
		createImagesTable,
	}

	// Tabellen erstellen
	for _, table := range tables {
		if err := execQuery(db, table); err != nil {
			return nil, err
		}
	}

	// Überprüfe, ob bereits Einstellungen in der settings-Tabelle existieren
	var count int
	db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count)

	// Wenn keine Einstellungen vorhanden sind, erstelle Standardeinstellungen
	if count == 0 {
		_, err = db.Exec(`
			INSERT INTO settings (
				id, docker_ip, use_custom_services, nodered_url, influxdb_url,
				use_external_broker, broker_url, broker_port, broker_username, broker_password
			) VALUES (
				1, '192.168.0.84', false, ':7777', ':8086', false, '', '', '', ''
			)
		`)
		if err != nil {
			logrus.Errorf("Fehler beim Erstellen der Standardeinstellungen: %v", err)
		}
	}

	// Check if there are any users in the database
	var countUsers int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&countUsers)

	// If no users are found, create a default admin user
	if countUsers == 0 {
		db.Exec(`
            INSERT INTO users (username, password, name, address, company, email)
            VALUES (?, ?, ?, ?, ?, ?)
        `, "admin", "password", "Admin", "Admin Street", "Admin Corp.", "admin@admin.com")
	}

	// Check if there are any broker settings in the database
	var countBrokerSettings int
	db.QueryRow("SELECT COUNT(*) FROM broker_settings").Scan(&countBrokerSettings)

	// If no broker settings are found, create a default setting
	if countBrokerSettings == 0 {
		db.Exec(`
            INSERT INTO broker_settings (address, username, password)
            VALUES (?, ?, ?)
        `, "ws://127.0.0.1:5001", "admin", "abc+1247")
	}

	// Check if there are any InfluxDB settings in the database
	var countInfluxDB int
	db.QueryRow("SELECT COUNT(*) FROM influxdb").Scan(&countInfluxDB)

	// If no InfluxDB settings are found, create a default setting
	if countInfluxDB == 0 {
		db.Exec(`
			INSERT INTO influxdb (url, token, org, bucket)
			VALUES (?, ?, ?, ?)
		`, "http://influxdb:8086", "secret-token", "idpm", "iot-data")
	}

	// Check if there are any settings in the settings table
	var countSettings int
	db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&countSettings)

	return db, nil
}

// execQuery führt eine SQL-Abfrage aus und behandelt Fehler
func execQuery(db *sql.DB, query string) error {
	_, err := db.Exec(query)
	if err != nil {
		logrus.Errorf("Failed to execute query: %v\nQuery: %s", err, query)
		return err
	}
	return nil
}
