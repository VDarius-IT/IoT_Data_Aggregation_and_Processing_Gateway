package logger

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var sugar *zap.SugaredLogger

func Init(levelStr, output, file string) {
    level := zapcore.InfoLevel
    switch levelStr {
    case "debug":
        level = zapcore.DebugLevel
    case "error":
        level = zapcore.ErrorLevel
    }

    cfg := zap.NewProductionConfig()
    cfg.Level = zap.NewAtomicLevelAt(level)
    if output == "stdout" {
        cfg.OutputPaths = []string{"stdout"}
    } else {
        cfg.OutputPaths = []string{file}
    }

    logger, _ := cfg.Build()
    sugar = logger.Sugar()
}

func Sync() {
    if sugar != nil {
        _ = sugar.Sync()
    }
}

func Sugar() *zap.SugaredLogger {
    return sugar
}
