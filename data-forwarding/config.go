package dataforwarding

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// WriterConfig enthält alle konfigurierbaren Parameter
type WriterConfig struct {
	// Buffer-Konfiguration
	MinBufferCapacity     int
	MaxBufferCapacity     int
	InitialBufferCapacity int
	TargetFlushInterval   time.Duration

	// Worker Pool
	WorkerCount int

	// Circuit Breaker
	FailureThreshold int
	RecoveryTimeout  time.Duration

	// Retry-Konfiguration
	MaxRetries int
	RetryDelay time.Duration

	// Health Check
	HealthCheckInterval time.Duration
	MQTTTimeout         time.Duration

	// Performance
	FlushTimeout      time.Duration
	ProcessingTimeout time.Duration
}

// LoadConfig lädt die Konfiguration aus Umgebungsvariablen
func LoadConfig() *WriterConfig {
	config := &WriterConfig{
		// Standardwerte
		MinBufferCapacity:     100,
		MaxBufferCapacity:     50000,
		InitialBufferCapacity: 1000,
		TargetFlushInterval:   5 * time.Second,
		WorkerCount:           10,
		FailureThreshold:      5,
		RecoveryTimeout:       30 * time.Second,
		MaxRetries:            3,
		RetryDelay:            5 * time.Second,
		HealthCheckInterval:   30 * time.Second,
		MQTTTimeout:           60 * time.Second,
		FlushTimeout:          10 * time.Second,
		ProcessingTimeout:     100 * time.Millisecond,
	}

	// Lade aus Umgebungsvariablen
	if val := os.Getenv("INFLUXDB_MIN_BUFFER_CAPACITY"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.MinBufferCapacity = intVal
		}
	}

	if val := os.Getenv("INFLUXDB_MAX_BUFFER_CAPACITY"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.MaxBufferCapacity = intVal
		}
	}

	if val := os.Getenv("INFLUXDB_INITIAL_BUFFER_CAPACITY"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.InitialBufferCapacity = intVal
		}
	}

	if val := os.Getenv("INFLUXDB_TARGET_FLUSH_INTERVAL_SEC"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.TargetFlushInterval = time.Duration(intVal) * time.Second
		}
	}

	if val := os.Getenv("INFLUXDB_WORKER_COUNT"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.WorkerCount = intVal
		}
	}

	if val := os.Getenv("INFLUXDB_FAILURE_THRESHOLD"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.FailureThreshold = intVal
		}
	}

	if val := os.Getenv("INFLUXDB_RECOVERY_TIMEOUT_SEC"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.RecoveryTimeout = time.Duration(intVal) * time.Second
		}
	}

	if val := os.Getenv("INFLUXDB_MAX_RETRIES"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.MaxRetries = intVal
		}
	}

	if val := os.Getenv("INFLUXDB_RETRY_DELAY_SEC"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.RetryDelay = time.Duration(intVal) * time.Second
		}
	}

	if val := os.Getenv("INFLUXDB_HEALTH_CHECK_INTERVAL_SEC"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.HealthCheckInterval = time.Duration(intVal) * time.Second
		}
	}

	if val := os.Getenv("INFLUXDB_MQTT_TIMEOUT_SEC"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.MQTTTimeout = time.Duration(intVal) * time.Second
		}
	}

	if val := os.Getenv("INFLUXDB_FLUSH_TIMEOUT_SEC"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.FlushTimeout = time.Duration(intVal) * time.Second
		}
	}

	if val := os.Getenv("INFLUXDB_PROCESSING_TIMEOUT_MS"); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			config.ProcessingTimeout = time.Duration(intVal) * time.Millisecond
		}
	}

	return config
}

// ValidateConfig prüft die Konfiguration auf Gültigkeit
func (c *WriterConfig) ValidateConfig() error {
	if c.MinBufferCapacity <= 0 {
		return fmt.Errorf("MinBufferCapacity muss größer als 0 sein")
	}
	if c.MaxBufferCapacity <= c.MinBufferCapacity {
		return fmt.Errorf("MaxBufferCapacity muss größer als MinBufferCapacity sein")
	}
	if c.InitialBufferCapacity < c.MinBufferCapacity || c.InitialBufferCapacity > c.MaxBufferCapacity {
		return fmt.Errorf("InitialBufferCapacity muss zwischen MinBufferCapacity und MaxBufferCapacity liegen")
	}
	if c.WorkerCount <= 0 {
		return fmt.Errorf("WorkerCount muss größer als 0 sein")
	}
	if c.FailureThreshold <= 0 {
		return fmt.Errorf("FailureThreshold muss größer als 0 sein")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries darf nicht negativ sein")
	}
	return nil
}
