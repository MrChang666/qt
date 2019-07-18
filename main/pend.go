package main

import (
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/MrChang666/qt/config"
	"github.com/MrChang666/qt/service"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/natefinch/lumberjack"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"strconv"
)

func initLog(logPath, logLevel string) {

	lumberjackLogger := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     3,
		LocalTime:  true,
	}

	log.SetFormatter(&log.TextFormatter{})

	mw := io.MultiWriter(lumberjackLogger, os.Stderr)

	log.SetOutput(mw)

	var level log.Level

	switch logLevel {
	case "debug":
		level = log.DebugLevel
	case "info":
		level = log.InfoLevel
	case "error":
		level = log.ErrorLevel
	default:
		level = log.DebugLevel
	}

	log.SetLevel(level)
}

func main() {

	cfg := config.InitConfig("pd", "./config")
	initLog(cfg.LogPath, cfg.LogLevel)
	fcClient := client.NewFCoinClient(cfg.SecretKey, cfg.AssKey, cfg.BaseUrl)

	start := make(chan int)

	for _, s := range cfg.Symbols {
		symbol := s["symbol"]
		balance, _ := decimal.NewFromString(s["balance"])
		assetPrecision, _ := strconv.Atoi(s["assetPrecision"])
		pricePrecision, _ := strconv.Atoi(s["pricePrecision"])

		maxRate, _ := decimal.NewFromString(s["maxRate"])
		minRate, _ := decimal.NewFromString(s["minRate"])
		midRate, _ := decimal.NewFromString(s["midRate"])
		minAsset, _ := decimal.NewFromString(s["minAsset"])
		period, _ := strconv.Atoi(s["period"])

		bySide := s["bySide"]
		ps := service.NewPendService(symbol, balance, midRate, maxRate, minRate, minAsset, int32(assetPrecision), int32(pricePrecision), fcClient, period, bySide)
		go ps.Run()
	}

	<-start
}
