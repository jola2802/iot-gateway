package logic

import (
	"database/sql"
	"os"
	"time"

	_ "github.com/glebarez/go-sqlite"
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

	createImagesTable = `
		CREATE TABLE IF NOT EXISTS images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device TEXT NOT NULL,
			process_id TEXT NOT NULL,
			image TEXT NOT NULL,
			timestamp TEXT NOT NULL
		);
	`

	createImageCaptureProcessesTable = `
		CREATE TABLE IF NOT EXISTS image_capture_processes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(100) NOT NULL,
			device_id INTEGER NOT NULL,
			endpoint TEXT NOT NULL,
			object_id TEXT NOT NULL,
			method_id TEXT NOT NULL,
			method_args TEXT,
			check_node_id TEXT NOT NULL,
			image_node_id TEXT NOT NULL,
			ack_node_id TEXT NOT NULL,
			enable_upload BOOLEAN DEFAULT 0,
			upload_url TEXT,
			upload_headers TEXT,
			timestamp_header_name TEXT,
			enable_cyclic BOOLEAN DEFAULT 0,
			cyclic_interval INTEGER DEFAULT 30,
			description TEXT,
			status TEXT DEFAULT 'stopped',
			last_execution TEXT,
			last_image TEXT,
			last_upload_status TEXT DEFAULT 'not_attempted',
			last_upload_error TEXT,
			upload_success_count INTEGER DEFAULT 0,
			upload_failure_count INTEGER DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (device_id) REFERENCES devices(id)
		);
	`

	createSystemSettingsTable = `
		CREATE TABLE IF NOT EXISTS system_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			setting_key VARCHAR(100) NOT NULL UNIQUE,
			setting_value TEXT,
			setting_type VARCHAR(50) DEFAULT 'string',
			description TEXT,
			category VARCHAR(50) DEFAULT 'general',
			is_encrypted BOOLEAN DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
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

	// SQLite-spezifische Konfiguration für bessere Concurrency
	db.SetMaxOpenConns(10)   // Mehr Verbindungen für verschachtelte Queries
	db.SetMaxIdleConns(2)    // Zwei Idle-Verbindungen für bessere Performance
	db.SetConnMaxLifetime(0) // Verbindungen niemals schließen

	// WAL-Mode aktivieren für bessere Concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}

	// Busy-Timeout setzen (30 Sekunden)
	if _, err := db.Exec("PRAGMA busy_timeout=30000"); err != nil {
		return nil, err
	}

	// Synchronous-Mode für bessere Performance
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, err
	}

	// Cache-Größe erhöhen
	if _, err := db.Exec("PRAGMA cache_size=10000"); err != nil {
		return nil, err
	}

	// SQL-Tabellen in einer Liste speichern
	tables := []string{
		createAuthTable,
		createACLTable,
		createUsersTable,
		createDevicesTable,
		createS7DatapointsTable,
		createOPCUADatanodesTable,
		createImagesTable,
		createImageCaptureProcessesTable,
		createSystemSettingsTable,
	}

	// Tabellen erstellen
	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return nil, err
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

	// Check if there are any system settings in the database
	var countSettings int
	db.QueryRow("SELECT COUNT(*) FROM system_settings").Scan(&countSettings)

	// If no settings are found, create default settings
	if countSettings == 0 {
		now := time.Now().Format("2006-01-02 15:04:05")

		defaultSettings := []struct {
			key, value, settingType, description, category string
		}{
			{"node_red_url", "http://node-red:1880", "string", "Node-RED Web-Interface URL", "integration"},
			{"influxdb_url", "http://influxdb:8086", "string", "InfluxDB Server URL", "integration"},
		}

		for _, setting := range defaultSettings {
			db.Exec(`
				INSERT INTO system_settings (setting_key, setting_value, setting_type, description, category, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`, setting.key, setting.value, setting.settingType, setting.description, setting.category, now, now)
		}
	}

	return db, nil
}
