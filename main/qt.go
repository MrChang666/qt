package main

import (
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/MrChang666/qt/config"
	"github.com/MrChang666/qt/service"
	"github.com/natefinch/lumberjack"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"strconv"

	"io"
	"os"
)

func initLog(logPath, logLevel string) {

	lumberjackLogger := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     3,
		LocalTime:  true,
	}

	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.TextFormatter{})

	mw := io.MultiWriter(lumberjackLogger, os.Stderr)

	log.SetOutput(mw)

	// Only log the warning severity or above.
	var level log.Level

	switch logLevel {
	case "debug":
		level = log.DebugLevel
	case "info":
		level = log.DebugLevel
	case "error":
		level = log.ErrorLevel
	default:
		level = log.DebugLevel
	}

	log.SetLevel(level)
}

func main() {
	cfg := config.InitConfig("qt", "./config")
	initLog(cfg.LogPath, cfg.LogLevel)
	fcClient := client.NewFCoinClient(cfg.SecretKey, cfg.AssKey, cfg.BaseUrl)

	start := make(chan int)

	for _, s := range cfg.Symbols {
		symbol := s["symbol"]
		balance, _ := decimal.NewFromString(s["balance"])
		minBalance, _ := decimal.NewFromString(s["minBalance"])
		minAsset, _ := decimal.NewFromString(s["minAsset"])
		assetPrecision, _ := strconv.Atoi(s["assetPrecision"])
		pricePrecison, _ := strconv.Atoi(s["pricePrecison"])

		buyLevel, _ := strconv.Atoi(s["buyLevel"])
		sellLevel, _ := strconv.Atoi(s["sellLevle"])
		period, _ := strconv.Atoi(s["period"])

		bySide := s["bySide"]

		ds := service.NewDigService(symbol, balance, minBalance, minAsset, int32(assetPrecision), int32(pricePrecison), fcClient, buyLevel, sellLevel, period, bySide)
		go ds.Run()
	}

	<-start
}
