package dataforwarding

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	MQTT "github.com/mochi-mqtt/server/v2"
	packets "github.com/mochi-mqtt/server/v2/packets"
	"github.com/sirupsen/logrus"
)

// TopicParserCache für vorverarbeitete Topic-Strukturen
type TopicParserCache struct {
	mu    sync.RWMutex
	cache map[string]*ParsedTopic
}

type ParsedTopic struct {
	DeviceID    string
	DatapointID string
	Measurement string
	IsValid     bool
}

// AdaptiveBuffer für dynamische Puffergröße
type AdaptiveBuffer struct {
	mu          sync.Mutex
	points      []*write.Point
	capacity    int
	minCapacity int
	maxCapacity int
	dataRate    float64
	lastFlush   time.Time
	flushCount  int64
}

// RetryQueue für fehlgeschlagene Schreiboperationen
type RetryQueue struct {
	mu         sync.Mutex
	queue      []*write.Point
	maxRetries int
}

// CircuitBreaker für robuste Fehlerbehandlung
type CircuitBreaker struct {
	mu              sync.RWMutex
	failureCount    int
	lastFailureTime time.Time
	state           CircuitState
	threshold       int
	timeout         time.Duration
}

type CircuitState int

const (
	Closed CircuitState = iota
	Open
	HalfOpen
)

// SystemHealth für Selbstheilung
type SystemHealth struct {
	mu                sync.RWMutex
	lastHealthyTime   time.Time
	consecutiveErrors int
	isHealthy         bool
	recoveryAttempts  int
}

// Globale Variablen
var (
	// Konfiguration
	writerConfig *WriterConfig

	// Topic-Parser-Cache
	topicCache = &TopicParserCache{
		cache: make(map[string]*ParsedTopic),
	}

	// Adaptive Buffer
	adaptiveBuffer *AdaptiveBuffer

	// Retry Queue
	retryQueue *RetryQueue

	// Circuit Breaker
	circuitBreaker *CircuitBreaker

	// System Health
	systemHealth = &SystemHealth{
		isHealthy: true,
	}

	// Performance-Metriken
	metrics = struct {
		processedPoints   int64
		failedPoints      int64
		flushOperations   int64
		avgProcessingTime int64
		lastFlushTime     time.Time
	}{}

	// InfluxDB Client
	client         influxdb2.Client
	writeAPI       api.WriteAPI
	influxConfig   *InfluxConfig
	subscriptionID = rand.Intn(100)

	// Worker Pool
	workerPool chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc

	// Selbstheilung
	lastMessageReceived time.Time
	healthCheckTicker   *time.Ticker
)

// Topic-Parser-Cache Methoden
func (tpc *TopicParserCache) GetParsedTopic(topic string) *ParsedTopic {
	tpc.mu.RLock()
	if parsed, exists := tpc.cache[topic]; exists {
		tpc.mu.RUnlock()
		return parsed
	}
	tpc.mu.RUnlock()

	// Parse und cache
	parsed := parseTopic(topic)
	tpc.mu.Lock()
	tpc.cache[topic] = parsed
	tpc.mu.Unlock()
	return parsed
}

func parseTopic(topic string) *ParsedTopic {
	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		return &ParsedTopic{IsValid: false}
	}

	deviceID := parts[2]
	measurement := parts[3]

	// Extrahiere DatapointID aus [DatapointId]_[DatapointName] Format
	datapointID := ""
	if strings.Contains(measurement, "[") && strings.Contains(measurement, "]") {
		start := strings.Index(measurement, "[")
		end := strings.Index(measurement, "]")
		if start != -1 && end != -1 && end > start {
			datapointID = measurement[start+1 : end]
		}
	}

	return &ParsedTopic{
		DeviceID:    deviceID,
		DatapointID: datapointID,
		Measurement: measurement,
		IsValid:     true,
	}
}

// Adaptive Buffer Methoden
func (ab *AdaptiveBuffer) AddPoint(point *write.Point) bool {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	ab.points = append(ab.points, point)
	currentSize := len(ab.points)

	// Dynamische Kapazitätsanpassung basierend auf Datenrate
	if ab.shouldAdjustCapacity() {
		ab.adjustCapacity()
	}

	// Flush-Trigger basierend auf Größe und Zeit
	return currentSize >= ab.capacity || ab.shouldTimeFlush()
}

func (ab *AdaptiveBuffer) shouldAdjustCapacity() bool {
	now := time.Now()
	if ab.lastFlush.IsZero() {
		ab.lastFlush = now
		return false
	}

	timeDiff := now.Sub(ab.lastFlush).Seconds()
	if timeDiff < 10 { // Mindestens 10 Sekunden zwischen Anpassungen
		return false
	}

	// Berechne aktuelle Datenrate
	currentRate := float64(len(ab.points)) / timeDiff
	rateChange := (currentRate - ab.dataRate) / ab.dataRate

	return rateChange > 0.2 || rateChange < -0.2 // 20% Änderung
}

func (ab *AdaptiveBuffer) adjustCapacity() {
	now := time.Now()
	timeDiff := now.Sub(ab.lastFlush).Seconds()
	currentRate := float64(len(ab.points)) / timeDiff

	// Ziel: Konfigurierbare Flush-Intervalle
	targetFlushInterval := writerConfig.TargetFlushInterval.Seconds()
	targetCapacity := int(currentRate * targetFlushInterval)

	if targetCapacity < ab.minCapacity {
		targetCapacity = ab.minCapacity
	} else if targetCapacity > ab.maxCapacity {
		targetCapacity = ab.maxCapacity
	}

	oldCapacity := ab.capacity
	ab.capacity = targetCapacity
	ab.dataRate = currentRate
	ab.lastFlush = now

	logrus.Infof("Buffer-Kapazität angepasst: %d -> %d (Rate: %.2f points/sec)",
		oldCapacity, targetCapacity, currentRate)
}

func (ab *AdaptiveBuffer) shouldTimeFlush() bool {
	if ab.lastFlush.IsZero() {
		return false
	}
	return time.Since(ab.lastFlush) > writerConfig.TargetFlushInterval
}

func (ab *AdaptiveBuffer) GetPoints() []*write.Point {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	points := ab.points
	ab.points = make([]*write.Point, 0, ab.capacity)
	ab.lastFlush = time.Now()
	atomic.AddInt64(&metrics.flushOperations, 1)

	return points
}

// Retry Queue Methoden
func (rq *RetryQueue) AddPoint(point *write.Point, retryCount int) {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	if retryCount < rq.maxRetries {
		rq.queue = append(rq.queue, point)
		logrus.Warnf("InfluxDB-Writer: Punkt zur Retry-Queue hinzugefügt (Versuch %d/%d, Queue-Größe: %d)",
			retryCount+1, rq.maxRetries, len(rq.queue))
	} else {
		atomic.AddInt64(&metrics.failedPoints, 1)
		logrus.Errorf("InfluxDB-Writer: Punkt nach %d Versuchen endgültig verworfen - Datenverlust!", rq.maxRetries)
	}
}

func (rq *RetryQueue) GetPoints() []*write.Point {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	points := rq.queue
	rq.queue = make([]*write.Point, 0)
	return points
}

// Circuit Breaker Methoden
func (cb *CircuitBreaker) Execute(operation func() error) error {
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker is open")
	}

	err := operation()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case Closed:
		return true
	case Open:
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = HalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case HalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		logrus.Warnf("InfluxDB-Writer: Circuit Breaker Fehler #%d/%d: %v",
			cb.failureCount, cb.threshold, err)

		if cb.failureCount >= cb.threshold {
			cb.state = Open
			logrus.Errorf("InfluxDB-Writer: Circuit Breaker geöffnet nach %d Fehlern - InfluxDB-Verbindung deaktiviert", cb.failureCount)
		}
	} else {
		if cb.state == HalfOpen {
			cb.state = Closed
			cb.failureCount = 0
			logrus.Info("InfluxDB-Writer: Circuit Breaker geschlossen - InfluxDB-Verbindung wieder aktiv")
		}
	}
}

// System Health Methoden
func (sh *SystemHealth) RecordError() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.consecutiveErrors++
	if sh.consecutiveErrors >= 3 {
		sh.isHealthy = false
		logrus.Warn("System als ungesund markiert")
	}
}

func (sh *SystemHealth) RecordSuccess() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.consecutiveErrors = 0
	sh.isHealthy = true
	sh.lastHealthyTime = time.Now()
	sh.recoveryAttempts = 0
}

func (sh *SystemHealth) IsHealthy() bool {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.isHealthy
}

func (sh *SystemHealth) AttemptRecovery() bool {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.isHealthy {
		return true
	}

	sh.recoveryAttempts++
	if sh.recoveryAttempts > 5 {
		logrus.Error("Maximale Wiederherstellungsversuche erreicht")
		return false
	}

	logrus.Infof("Wiederherstellungsversuch %d/5", sh.recoveryAttempts)
	return true
}

// Konfiguration
func setInfluxDBConfig() {
	influxdbURL := os.Getenv("INFLUXDB_URL")
	influxdbToken := os.Getenv("INFLUXDB_TOKEN")
	influxdbOrg := os.Getenv("INFLUXDB_ORG")
	influxdbBucket := os.Getenv("INFLUXDB_BUCKET")

	if influxdbURL == "" || influxdbToken == "" || influxdbOrg == "" || influxdbBucket == "" {
		logrus.Warn("Mindestens eine InfluxDB-Umgebungsvariable ist nicht gesetzt. Verwende Standardwerte.")
		influxdbURL = "http://influxdb:8086"
		influxdbToken = "secret-token"
		influxdbOrg = "idpm"
		influxdbBucket = "iot-data"
	}

	influxConfig = &InfluxConfig{
		URL:    influxdbURL,
		Token:  influxdbToken,
		Org:    influxdbOrg,
		Bucket: influxdbBucket,
	}
}

func GetInfluxConfig(db *sql.DB) (*InfluxConfig, error) {
	if influxConfig == nil {
		setInfluxDBConfig()
	}

	if influxConfig == nil || influxConfig.URL == "" {
		return nil, fmt.Errorf("Keine InfluxDB-Konfiguration gefunden")
	}

	// Teste Verbindung
	testClient := influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	defer testClient.Close()

	health, err := testClient.Health(context.Background())
	if err != nil || health.Status != "pass" {
		return nil, fmt.Errorf("InfluxDB nicht erreichbar: %v", err)
	}

	return influxConfig, nil
}

// Client-Initialisierung mit Selbstheilung
func initializeClient() error {
	if client != nil {
		// Teste bestehende Verbindung
		health, err := client.Health(context.Background())
		if err == nil && health.Status == "pass" {
			return nil
		}
		// Verbindung ist ungesund, schließe sie
		client.Close()
		client = nil
		writeAPI = nil
	}

	// Warte auf gültige Konfiguration
	for influxConfig == nil || influxConfig.URL == "" {
		if influxConfig == nil {
			setInfluxDBConfig()
		}
		if influxConfig == nil || influxConfig.URL == "" {
			logrus.Warn("Keine gültige InfluxDB-Konfiguration, versuche in 10 Sekunden erneut...")
			time.Sleep(10 * time.Second)
		}
	}

	client = influxdb2.NewClient(influxConfig.URL, influxConfig.Token)
	writeAPI = client.WriteAPI(influxConfig.Org, influxConfig.Bucket)

	logrus.Info("InfluxDB-Client erfolgreich initialisiert")
	return nil
}

// Worker Pool für parallele Verarbeitung
func startWorkerPool() {
	workerPool = make(chan struct{}, writerConfig.WorkerCount)

	// Fülle Worker Pool
	for i := 0; i < writerConfig.WorkerCount; i++ {
		workerPool <- struct{}{}
	}
}

// Intelligente Datenverarbeitung
func processDataPoint(pk packets.Packet) (*write.Point, error) {
	start := time.Now()
	defer func() {
		processingTime := time.Since(start).Microseconds()
		atomic.StoreInt64(&metrics.avgProcessingTime, processingTime)
	}()

	// Verwende Topic-Cache
	parsedTopic := topicCache.GetParsedTopic(pk.TopicName)
	if !parsedTopic.IsValid {
		return nil, fmt.Errorf("ungültiges Topic-Format: %s", pk.TopicName)
	}

	// Effiziente Payload-Verarbeitung
	fieldValue := processPayloadOptimized(pk.Payload)

	// Erstelle InfluxDB-Punkt
	point := influxdb2.NewPointWithMeasurement(parsedTopic.Measurement).
		AddTag("deviceId", parsedTopic.DeviceID).
		AddTag("datapointId", parsedTopic.DatapointID).
		AddField("value", fieldValue).
		SetTime(time.Now())

	atomic.AddInt64(&metrics.processedPoints, 1)
	return point, nil
}

// Optimierte Payload-Verarbeitung
func processPayloadOptimized(payload []byte) interface{} {
	payloadStr := strings.TrimSpace(string(payload))

	// Schnelle Integer-Prüfung
	if len(payloadStr) > 0 && payloadStr[0] >= '0' && payloadStr[0] <= '9' {
		if intVal, err := strconv.ParseInt(payloadStr, 10, 64); err == nil {
			return float64(intVal)
		}
		if floatVal, err := strconv.ParseFloat(payloadStr, 64); err == nil {
			return floatVal
		}
	}

	return payloadStr
}

// Robuste Flush-Operation mit Circuit Breaker
func flushBufferWithRetry(db *sql.DB) error {
	points := adaptiveBuffer.GetPoints()
	if len(points) == 0 {
		return nil
	}

	// Verarbeite Retry-Queue zuerst
	retryPoints := retryQueue.GetPoints()
	if len(retryPoints) > 0 {
		points = append(retryPoints, points...)
		logrus.Infof("Verarbeite %d Punkte (davon %d aus Retry-Queue)", len(points), len(retryPoints))
	}

	return circuitBreaker.Execute(func() error {
		// Stelle sicher, dass Client verfügbar ist
		if err := initializeClient(); err != nil {
			return err
		}

		// Batch-Schreiben mit Fehlerbehandlung
		for _, point := range points {
			writeAPI.WritePoint(point)
		}

		// Flush mit Timeout
		flushCtx, cancel := context.WithTimeout(context.Background(), writerConfig.FlushTimeout)
		defer cancel()

		writeAPI.Flush()

		// Warte auf Flush-Abschluss
		select {
		case <-flushCtx.Done():
			return fmt.Errorf("flush timeout")
		default:
			// Erfolgreich
		}

		systemHealth.RecordSuccess()
		// logrus.Infof("Erfolgreich %d Punkte in InfluxDB geschrieben", len(points))
		return nil
	})
}

// Selbstheilungs-Mechanismus
func startSelfHealing() {
	healthCheckTicker = time.NewTicker(writerConfig.HealthCheckInterval)
	go func() {
		for {
			select {
			case <-healthCheckTicker.C:
				performHealthCheck()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func performHealthCheck() {
	// Prüfe System-Gesundheit
	if !systemHealth.IsHealthy() {
		if !systemHealth.AttemptRecovery() {
			logrus.Error("System-Wiederherstellung fehlgeschlagen")
			return
		}

		// Versuche Client-Neustart
		logrus.Info("Versuche Client-Neustart...")
		if client != nil {
			client.Close()
			client = nil
			writeAPI = nil
		}

		if err := initializeClient(); err != nil {
			logrus.Errorf("Client-Neustart fehlgeschlagen: %v", err)
			systemHealth.RecordError()
			return
		}

		logrus.Info("Client-Neustart erfolgreich")
	}

	// Prüfe MQTT-Verbindung
	if time.Since(lastMessageReceived) > writerConfig.MQTTTimeout {
		logrus.Warnf("Keine MQTT-Nachrichten in den letzten %v", writerConfig.MQTTTimeout)
		// Hier könnte ein MQTT-Reconnect implementiert werden
	}
}

// Optimierter MQTT-Callback
func influxMessageCallback(db *sql.DB) func(client *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
	return func(client *MQTT.Client, sub packets.Subscription, pk packets.Packet) {
		lastMessageReceived = time.Now()

		// Worker aus Pool holen
		select {
		case <-workerPool:
			// Worker verfügbar
		case <-time.After(writerConfig.ProcessingTimeout):
			// Timeout - verarbeite synchron
			logrus.Warn("Worker Pool voll, verarbeite synchron")
		}

		// Verarbeite Datenpunkt
		point, err := processDataPoint(pk)
		if err != nil {
			logrus.Errorf("Fehler bei Datenpunkt-Verarbeitung: %v", err)
			workerPool <- struct{}{} // Worker zurückgeben
			return
		}

		// Füge Punkt zum Buffer hinzu
		if adaptiveBuffer.AddPoint(point) {
			// Flush auslösen
			go func() {
				if err := flushBufferWithRetry(db); err != nil {
					logrus.Errorf("Flush-Fehler: %v", err)
					systemHealth.RecordError()

					// Füge fehlgeschlagene Punkte zur Retry-Queue hinzu
					points := adaptiveBuffer.GetPoints()
					for _, p := range points {
						retryQueue.AddPoint(p, 0)
					}
				}
				workerPool <- struct{}{} // Worker zurückgeben
			}()
		} else {
			workerPool <- struct{}{} // Worker zurückgeben
		}
	}
}

// Hauptfunktionen
func StartInfluxDBWriter(db *sql.DB, server *MQTT.Server) {
	// Lade und validiere Konfiguration
	writerConfig = LoadConfig()
	if err := writerConfig.ValidateConfig(); err != nil {
		logrus.Fatalf("Ungültige Konfiguration: %v", err)
	}

	// Initialisiere Komponenten mit Konfiguration
	adaptiveBuffer = &AdaptiveBuffer{
		capacity:    writerConfig.InitialBufferCapacity,
		minCapacity: writerConfig.MinBufferCapacity,
		maxCapacity: writerConfig.MaxBufferCapacity,
		points:      make([]*write.Point, 0, writerConfig.InitialBufferCapacity),
	}

	retryQueue = &RetryQueue{
		maxRetries: writerConfig.MaxRetries,
	}

	circuitBreaker = &CircuitBreaker{
		threshold: writerConfig.FailureThreshold,
		timeout:   writerConfig.RecoveryTimeout,
		state:     Closed,
	}

	ctx, cancel = context.WithCancel(context.Background())

	// Initialisierung
	setInfluxDBConfig()
	startWorkerPool()
	startSelfHealing()

	// Client initialisieren
	if err := initializeClient(); err != nil {
		logrus.Errorf("Fehler beim Initialisieren des InfluxDB-Clients: %v", err)
	}

	// MQTT-Subscription
	server.Subscribe("data/#", subscriptionID, influxMessageCallback(db))
	lastMessageReceived = time.Now()

	// Periodischer Flush für verbleibende Punkte
	go func() {
		ticker := time.NewTicker(writerConfig.TargetFlushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := flushBufferWithRetry(db); err != nil {
					logrus.Errorf("Periodischer Flush-Fehler: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Periodisches Logging der Metriken
	go func() {
		metricsTicker := time.NewTicker(60 * time.Second) // Alle 60 Sekunden
		defer metricsTicker.Stop()

		for {
			select {
			case <-metricsTicker.C:
				LogMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()

	logrus.Infof("InfluxDB-Writer mit Selbstheilung gestartet (Worker: %d, Buffer: %d-%d)",
		writerConfig.WorkerCount, writerConfig.MinBufferCapacity, writerConfig.MaxBufferCapacity)
}

func StopInfluxDBWriter() {
	if cancel != nil {
		cancel()
	}

	if healthCheckTicker != nil {
		healthCheckTicker.Stop()
	}

	// Finaler Flush
	if err := flushBufferWithRetry(nil); err != nil {
		logrus.Errorf("Finaler Flush-Fehler: %v", err)
	}

	if client != nil {
		client.Close()
		client = nil
		writeAPI = nil
	}

	logrus.Info("InfluxDB-Writer gestoppt")
}

// LogMetrics gibt aktuelle Metriken über Logrus aus
func LogMetrics() {
	processed := atomic.LoadInt64(&metrics.processedPoints)
	failed := atomic.LoadInt64(&metrics.failedPoints)
	flushOps := atomic.LoadInt64(&metrics.flushOperations)
	avgTime := atomic.LoadInt64(&metrics.avgProcessingTime)
	bufferSize := 0
	retrySize := 0

	if adaptiveBuffer != nil {
		bufferSize = len(adaptiveBuffer.points)
	}
	if retryQueue != nil {
		retrySize = len(retryQueue.queue)
	}

	// Berechne Erfolgsrate
	successRate := 0.0
	if processed > 0 {
		successRate = float64(processed-failed) / float64(processed) * 100
	}

	logrus.Infof("InfluxDB-Writer Metriken: Verarbeitet=%d, Fehler=%d, Erfolgsrate=%.1f%%, Flush-Operationen=%d, Durchschnittszeit=%dμs, Buffer=%d, Retry-Queue=%d",
		processed, failed, successRate, flushOps, avgTime, bufferSize, retrySize)

	// Warnung bei hoher Fehlerrate
	if successRate < 97.0 && processed > 100 {
		logrus.Warnf("Hohe Fehlerrate im InfluxDB-Writer: %.1f%% (Verarbeitet: %d, Fehler: %d)",
			successRate, processed, failed)
	}

	// Warnung bei vollem Buffer
	if bufferSize > writerConfig.MaxBufferCapacity*8/10 {
		logrus.Warnf("Buffer fast voll: %d/%d Punkte", bufferSize, writerConfig.MaxBufferCapacity)
	}

	// Warnung bei Retry-Queue
	if retrySize > 0 {
		logrus.Warnf("Retry-Queue enthält %d Punkte", retrySize)
	}
}
