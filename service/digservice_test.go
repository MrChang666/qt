package service

import (
	"fmt"
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/shopspring/decimal"
	"testing"
)

func initClient() *client.FCoinClient {
	baseUrl := "https://api.fcoin.com/v2"
	assKey := ""
	secretKey := ""
	return client.NewFCoinClient(secretKey, assKey, baseUrl)
}

func TestDigService_Run(t *testing.T) {
	fc := initClient()
	depth, err := fc.GetDepth("btcusdt", "L20")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(len(depth.Data.Bids))
	fmt.Println(depth.Data.Bids)
}

func TestDe(t *testing.T) {
	p := decimal.New(1, 4)

	balance := decimal.New(100, 0)
	buyPrice := decimal.NewFromFloat(9216.4)
	assetAmt := balance.Div(buyPrice)
	fmt.Println(assetAmt)
	assetAmt = assetAmt.Mul(p).Floor().Div(p)
	fmt.Println(assetAmt)
}

func TestCrateOrder(t *testing.T) {
	fc := initClient()
	buyPrice := decimal.NewFromFloat(9232.1)
	balance := decimal.New(100, 0)
	p := decimal.New(1, 4)

	assetAmt := balance.Div(buyPrice)
	assetAmt = assetAmt.Mul(p).Floor().Div(p)
	newOrder := &client.NewOrder{
		Amount:    assetAmt.String(),
		OrderType: "limit", //限价limit 市价 market
		Exchange:  "main",  //主板
		Side:      "buy",   //sell buy
		Symbol:    "btcusdt",
		Price:     buyPrice.String(),
	}
	res, err := fc.CreateOrder(newOrder)
	fmt.Println(res)
	if err != nil {
		t.Fatal(err)
	}
}
