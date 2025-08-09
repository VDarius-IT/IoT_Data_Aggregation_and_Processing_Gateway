package server

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/your-username/iot-edge-gateway/internal/config"
    "github.com/your-username/iot-edge-gateway/internal/logger"
    "github.com/your-username/iot-edge-gateway/internal/buffer"
    "github.com/your-username/iot-edge-gateway/internal/mqtt"
    "github.com/your-username/iot-edge-gateway/internal/kafka"
    "github.com/your-username/iot-edge-gateway/internal/forwarder"
    "github.com/your-username/iot-edge-gateway/internal/metrics"
)

type Server struct {
    cfg *config.Config
    http *http.Server
    ctx context.Context
    cancel context.CancelFunc

    store      *buffer.Store
    mqttClient *mqtt.Client
    producer   *kafka.Producer
    fwd        *forwarder.Forwarder
}

// New initializes the server, metrics endpoint and components (buffer, mqtt, kafka, forwarder).
func New(cfg *config.Config) (*Server, error) {
    ctx, cancel := context.WithCancel(context.Background())
    s := &Server{
        cfg:    cfg,
        ctx:    ctx,
        cancel: cancel,
    }

    // Initialize metrics
    metrics.Init()

    mux := http.NewServeMux()
    mux.Handle("/metrics", promhttp.Handler())
    addr := cfg.Server.MetricsAddr
    s.http = &http.Server{
        Addr:    addr,
        Handler: mux,
    }

    // Initialize buffer store
    bufPath := ""
    if cfg != nil && cfg.Buffer != nil {
        if v, ok := cfg.Buffer["path"]; ok {
            bufPath = fmt.Sprint(v)
        }
    }
    if bufPath == "" {
        bufPath = "./data/buffer.db"
    }
    store, err := buffer.Init(bufPath)
    if err != nil {
        return nil, fmt.Errorf("buffer init: %w", err)
    }
    s.store = store

    // Initialize Kafka producer (simple handling - expects brokers string or list)
    var brokers string
    if cfg != nil && cfg.Kafka != nil {
        if v, ok := cfg.Kafka["brokers"]; ok {
            // Try to handle slice or string
            switch vv := v.(type) {
            case []interface{}:
                for i, item := range vv {
                    if i > 0 {
                        brokers += ","
                    }
                    brokers += fmt.Sprint(item)
                }
            default:
                brokers = fmt.Sprint(v)
            }
        }
    }
    topic := "iot-sensor-data"
    if cfg != nil && cfg.Kafka != nil {
        if v, ok := cfg.Kafka["topic"]; ok {
            topic = fmt.Sprint(v)
        }
    }
    clientID := ""
    if cfg != nil && cfg.Kafka != nil {
        if v, ok := cfg.Kafka["client_id"]; ok {
            clientID = fmt.Sprint(v)
        }
    }

    if brokers != "" {
        prod, err := kafka.NewProducer(brokers, topic, clientID)
        if err != nil {
            s.store.Close()
            return nil, fmt.Errorf("kafka producer init: %w", err)
        }
        s.producer = prod
    } else {
        // No brokers configured - producer will be nil and forwarder will be a no-op
        fmt.Println("kafka brokers not configured; forwarder will be disabled")
    }

    // Initialize MQTT client if broker is configured
    mqttBroker := ""
    mqttTopic := ""
    mqttClientID := ""
    if cfg != nil && cfg.MQTT != nil {
        if v, ok := cfg.MQTT["broker"]; ok {
            mqttBroker = fmt.Sprint(v)
        }
        if v, ok := cfg.MQTT["topic"]; ok {
            mqttTopic = fmt.Sprint(v)
        }
        if v, ok := cfg.MQTT["client_id"]; ok {
            mqttClientID = fmt.Sprint(v)
        }
    }
    if mqttTopic == "" {
        mqttTopic = "sensors/#"
    }
    if mqttBroker != "" {
        mc, err := mqtt.New(mqttBroker, mqttClientID, mqttTopic, 1, s.store)
        if err != nil {
            if s.producer != nil {
                s.producer.Close()
            }
            s.store.Close()
            return nil, fmt.Errorf("mqtt client init: %w", err)
        }
        s.mqttClient = mc
    } else {
        fmt.Println("mqtt broker not configured; mqtt consumer disabled")
    }

    // Create forwarder only if producer exists
    flushInterval := 30 * time.Second
    if cfg != nil && cfg.Buffer != nil {
        if v, ok := cfg.Buffer["flush_interval_seconds"]; ok {
            if fv, ok := v.(int); ok && fv > 0 {
                flushInterval = time.Duration(fv) * time.Second
            }
        }
    }
    if s.producer != nil {
        fwd := forwarder.New(s.store, s.producer, flushInterval, 3, 5*time.Second)
        s.fwd = fwd
    }

    return s, nil
}

func (s *Server) Start() error {
    logger.Sugar().Infof("starting metrics server on %s", s.http.Addr)
    go func() {
        if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Sugar().Errorf("metrics server error: %v", err)
        }
    }()

    // Start forwarder if configured
    if s.fwd != nil {
        s.fwd.Start()
        logger.Sugar().Info("forwarder started")
    }

    // MQTT client was subscribed in New(); nothing to start explicitly here.
    <-s.ctx.Done()
    return nil
}

func (s *Server) Stop() {
    s.cancel()

    // Stop forwarder first
    if s.fwd != nil {
        s.fwd.Stop()
    }

    // Close MQTT client
    if s.mqttClient != nil {
        s.mqttClient.Close()
    }

    // Close producer
    if s.producer != nil {
        s.producer.Close()
    }

    // Close store
    if s.store != nil {
        _ = s.store.Close()
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = s.http.Shutdown(ctx)
    logger.Sugar().Info("server stopped")
}
