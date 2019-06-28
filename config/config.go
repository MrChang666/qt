package config

import (
	"fmt"
	"github.com/spf13/viper"
)

type Config struct {
	LogPath   string
	LogLevel  string
	BaseUrl   string
	AssKey    string
	SecretKey string
	Symbols   []map[string]string
	Dsn       string
}

func InitConfig(cfgName, cfgPath string) *Config {
	viper.SetConfigName(cfgName)
	viper.AddConfigPath(cfgPath)
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}

	ss := make([]map[string]string, 0, 8)

	symbols := viper.Get("symbols")

	for _, val := range symbols.([]interface{}) {
		maps := make(map[string]string)
		for k, v := range val.(map[interface{}]interface{}) {
			maps[k.(string)] = v.(string)
		}
		ss = append(ss, maps)
	}

	cfg := &Config{
		LogPath:   viper.GetString("logPath"),
		LogLevel:  viper.GetString("logLevel"),
		BaseUrl:   viper.GetString("baseUrl"),
		AssKey:    viper.GetString("assKey"),
		SecretKey: viper.GetString("secretKey"),
		Symbols:   ss,
		Dsn:       viper.GetString("dsn"),
	}

	return cfg
}
