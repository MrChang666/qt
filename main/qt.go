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
		level = log.InfoLevel
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

		buyLevel, _ := strconv.Atoi(s["buyLevel"])
		sellLevel, _ := strconv.Atoi(s["sellLevel"])
		period, _ := strconv.Atoi(s["period"])

		bySide := s["bySide"]
		ds := service.NewDigService(symbol, balance, minBalance, minAsset, int32(assetPrecision), int32(pricePrecision), fcClient, buyLevel, sellLevel, period, bySide, dbSource)
		go ds.Run()
		go ds.SaveOrder()
	}

	<-start
}
