package service

import (
	"fmt"
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/shopspring/decimal"
	"log"
)

type DigService struct {
	symbol         string          //交易对，如btcusdt
	balance        decimal.Decimal //允许使用的金额，比如100
	assetPrecison  int
	pricepPrecison int
}

func NewDigService(symbol string, balance decimal.Decimal, assetPrecison int, pricepPrecison int) *DigService {
	ds := &DigService{
		symbol:         symbol,
		balance:        balance,
		assetPrecison:  assetPrecison,
		pricepPrecison: pricepPrecison,
	}
	return ds
}

func (ds *DigService) Run() {
	//assKey: 52ff5164d7754cc1b317263368d38286
	//secretKey: d523f98514a640ab980ca2a72e9197f3
	//https://api.fcoin.com/v2/
	baseUrl := "https://api.fcoin.com/v2"
	assKey := ""
	secretKey := ""
	fclient := client.NewFCoinClient(secretKey, assKey, baseUrl)
	currency := "usdt"
	available, err := fclient.GetAvailableBalance(currency)
	if err != nil {
		log.Println(err)
	}
	fmt.Println(available)
}
