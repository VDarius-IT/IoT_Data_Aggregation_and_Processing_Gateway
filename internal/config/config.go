package config

import (
    "github.com/spf13/viper"
)

type Config struct {
    MQTT       map[string]interface{} `mapstructure:"mqtt"`
    Kafka      map[string]interface{} `mapstructure:"kafka"`
    Buffer     map[string]interface{} `mapstructure:"buffer"`
    Processing map[string]interface{} `mapstructure:"processing"`
    Logging    struct {
        Level  string `mapstructure:"level"`
        Output string `mapstructure:"output"`
        File   string `mapstructure:"file"`
    } `mapstructure:"logging"`
    Server struct {
        MetricsAddr string `mapstructure:"metrics_addr"`
    } `mapstructure:"server"`
}

func Load(path string) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(path)
    if err := v.ReadInConfig(); err != nil {
        return nil, err
    }
    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
