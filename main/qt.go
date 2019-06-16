package main

import (
	"github.com/MrChang666/qt/service"
	"github.com/shopspring/decimal"
)

func main() {
	balance := decimal.New(100, 0)
	ds := service.NewDigService("btcusdt", balance, 4, 1)
	ds.Run()

}
