package bootstrap

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Outbox   OutboxConfig
	Kafka    KafkaConfig
	Tracing  TracingConfig
	Log      LogConfig
	EventBus EventBusConfig
	SigNoz   SigNozConfig
}

type ServerConfig struct {
	Port            int
	ShutdownTimeout time.Duration
	HTTPS           HTTPSConfig
}

type HTTPSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

type OutboxConfig struct {
	BatchSize    int
	PollInterval time.Duration
	MaxRetries   int
}

type KafkaConfig struct {
	Brokers       []string
	Topic         string
	ConsumerGroup string
	BatchSize     int
	BatchTimeout  time.Duration
}

type TracingConfig struct {
	OTLPEndpoint string
	ServiceName  string
}

type LogConfig struct {
	Level    string
	FilePath string
}

type EventBusConfig struct {
	// Type selects the event bus implementation: "inmemory" or "kafka".
	Type string
}

type SigNozConfig struct {
	// OTLPEndpoint is the SigNoz OTel Collector address (e.g. "localhost:4318").
	// This is separate from the local OTel Collector used for Jaeger traces.
	OTLPEndpoint string
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:            envInt("SERVER_PORT", 8080),
			ShutdownTimeout: envDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second),
			HTTPS: HTTPSConfig{
				Enabled:  envBool("HTTPS_ENABLED", false),
				CertFile: envStr("HTTPS_CERT_FILE", ""),
				KeyFile:  envStr("HTTPS_KEY_FILE", ""),
			},
		},
		Database: DatabaseConfig{
			Host:     envStr("DB_HOST", "localhost"),
			Port:     envInt("DB_PORT", 5432),
			User:     envStr("DB_USER", "order"),
			Password: envStr("DB_PASSWORD", "order123"),
			DBName:   envStr("DB_NAME", "orderdb"),
			SSLMode:  envStr("DB_SSLMODE", "disable"),
		},
		Outbox: OutboxConfig{
			BatchSize:    envInt("OUTBOX_BATCH_SIZE", 50),
			PollInterval: envDuration("OUTBOX_POLL_INTERVAL", 1*time.Second),
			MaxRetries:   envInt("OUTBOX_MAX_RETRIES", 10),
		},
		Kafka: KafkaConfig{
			Brokers:       envStrSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			Topic:         envStr("KAFKA_TOPIC", "order-events"),
			ConsumerGroup: envStr("KAFKA_CONSUMER_GROUP", "order-projection"),
			BatchSize:     envInt("KAFKA_BATCH_SIZE", 100),
			BatchTimeout:  envDuration("KAFKA_BATCH_TIMEOUT", 10*time.Millisecond),
		},
		Tracing: TracingConfig{
			OTLPEndpoint: envStr("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			ServiceName:  envStr("OTEL_SERVICE_NAME", "order-service"),
		},
		Log: LogConfig{
			Level:    envStr("LOG_LEVEL", "info"),
			FilePath: envStr("LOG_FILE_PATH", ""),
		},
		EventBus: EventBusConfig{
			Type: envStr("EVENT_BUS_TYPE", "inmemory"),
		},
		SigNoz: SigNozConfig{
			OTLPEndpoint: envStr("SIGNOZ_OTLP_ENDPOINT", ""),
		},
	}
}

func envStr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envStrSlice(key string, defaultVal []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func envBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		return strings.ToLower(v) == "true" || v == "1" || strings.ToLower(v) == "yes"
	}
	return defaultVal
}
