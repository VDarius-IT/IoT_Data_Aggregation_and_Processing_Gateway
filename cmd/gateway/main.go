package main

import (
    "flag"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/your-username/iot-edge-gateway/internal/config"
    "github.com/your-username/iot-edge-gateway/internal/logger"
    "github.com/your-username/iot-edge-gateway/internal/server"
)

func main() {
    cfgPath := flag.String("config", "config/config.yaml", "Path to config file")
    flag.Parse()

    cfg, err := config.Load(*cfgPath)
    if err != nil {
        log.Fatalf("failed loading config: %v", err)
    }

    logger.Init(cfg.Logging.Level, cfg.Logging.Output, cfg.Logging.File)
    defer logger.Sync()

    s, err := server.New(cfg)
    if err != nil {
        log.Fatalf("failed creating server: %v", err)
    }

    // graceful shutdown
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    go func() {
        if err := s.Start(); err != nil {
            log.Fatalf("server error: %v", err)
        }
    }()

    <-stop
    s.Stop()
}
