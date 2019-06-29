package main

import (
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/MrChang666/qt/config"
	"github.com/MrChang666/qt/service"
	"github.com/jinzhu/gorm"
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
		level = log.DebugLevel
	case "error":
		level = log.ErrorLevel
	default:
		level = log.DebugLevel
	}

	log.SetLevel(level)
}

func main() {
	cfg := config.InitConfig("mt", "./config")
	initLog(cfg.LogPath, cfg.LogLevel)
	fcClient := client.NewFCoinClient(cfg.SecretKey, cfg.AssKey, cfg.BaseUrl)

	dbSource, err := gorm.Open("mysql", cfg.Dsn)
	if err != nil {
		panic(err)
	}
	log.Debug("db inited")
	defer dbSource.Close()

	start := make(chan int)

	for _, s := range cfg.Symbols {
		symbol := s["symbol"]
		balance, _ := decimal.NewFromString(s["balance"])
		minBalance, _ := decimal.NewFromString(s["minBalance"])
		minAsset, _ := decimal.NewFromString(s["minAsset"])
		assetPrecision, _ := strconv.Atoi(s["assetPrecision"])
		pricePrecision, _ := strconv.Atoi(s["pricePrecision"])

		period, _ := strconv.Atoi(s["period"])

		bySide := s["bySide"]
		diffBuyRate, _ := decimal.NewFromString(s["diffBuyRate"])
		diffSellRate, _ := decimal.NewFromString(s["diffSellRate"])

		mt := service.NewMarketService(symbol, balance, minBalance, minAsset, diffBuyRate, diffSellRate, int32(assetPrecision), int32(pricePrecision), fcClient, period, bySide, dbSource)
		go mt.Run()
		go mt.SaveOrder()
	}

	<-start
}
