package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type JWTConfig struct {
	AuthMode   string
	HMACSecret string
	JWKSURL    string
	Issuer     string
	Audience   string
}

type Config struct {
	HTTPPort              string
	PostgresDSN           string
	KafkaBrokers          []string
	KafkaEmailSendTopic   string
	KafkaEmailResultTopic string
	KafkaConsumerGroup    string
	ChannelBufferSize     int
	JWT                   JWTConfig
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPPort:              getEnv("HTTP_PORT", "8080"),
		KafkaEmailSendTopic:   getEnv("KAFKA_EMAIL_SEND_TOPIC", "email.send"),
		KafkaEmailResultTopic: getEnv("KAFKA_EMAIL_RESULT_TOPIC", "email.result"),
		KafkaConsumerGroup:    getEnv("KAFKA_CONSUMER_GROUP", "notification-service"),
	}

	var errs []string

	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		cfg.PostgresDSN = v
	} else {
		errs = append(errs, "POSTGRES_DSN is required")
	}

	brokersRaw := os.Getenv("KAFKA_BROKERS")
	if brokersRaw == "" {
		errs = append(errs, "KAFKA_BROKERS is required")
	} else {
		cfg.KafkaBrokers = strings.Split(brokersRaw, ",")
	}

	bufSize, err := strconv.Atoi(getEnv("CHANNEL_BUFFER_SIZE", "1000"))
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid CHANNEL_BUFFER_SIZE: %v", err))
	}
	cfg.ChannelBufferSize = bufSize

	jwtMode := getEnv("JWT_AUTH_MODE", "symmetric")
	cfg.JWT = JWTConfig{
		AuthMode: jwtMode,
		Issuer:   os.Getenv("JWT_ISSUER"),
		Audience: os.Getenv("JWT_AUDIENCE"),
	}

	switch jwtMode {
	case "symmetric":
		if v := os.Getenv("JWT_HMAC_SECRET"); v != "" {
			cfg.JWT.HMACSecret = v
		} else {
			errs = append(errs, "JWT_HMAC_SECRET is required when JWT_AUTH_MODE=symmetric")
		}
	case "jwks":
		if v := os.Getenv("JWT_JWKS_URL"); v != "" {
			cfg.JWT.JWKSURL = v
		} else {
			errs = append(errs, "JWT_JWKS_URL is required when JWT_AUTH_MODE=jwks")
		}
	default:
		errs = append(errs, fmt.Sprintf("unsupported JWT_AUTH_MODE: %s (must be symmetric or jwks)", jwtMode))
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
