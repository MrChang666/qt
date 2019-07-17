package util

import (
	"fmt"
	"github.com/shopspring/decimal"
	"strings"

	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/MrChang666/qt/model"
	"github.com/jinzhu/gorm"
)

const (
	USDT = "usdt"
	PAX  = "pax"
)

func GetUSDT(symbol string) string {
	var usdt string
	if strings.HasSuffix(symbol, USDT) {
		usdt = USDT
	} else if strings.HasSuffix(symbol, PAX) {
		usdt = PAX
	}
	return usdt
}

func GetCurrency(symbol string) string {
	var cur string
	if strings.HasSuffix(symbol, USDT) {
		cur = strings.TrimSuffix(symbol, USDT)
	} else if strings.HasSuffix(symbol, PAX) {
		cur = strings.TrimSuffix(symbol, PAX)
	}
	return cur
}

func SaveOrderWithInfo(orderInfo *client.OrderInfo, db *gorm.DB) error {
	amount, _ := decimal.NewFromString(orderInfo.Data.Amount)
	price, _ := decimal.NewFromString(orderInfo.Data.Price)
	moi := &model.OrderInfo{
		Amount:    amount,
		OrderId:   orderInfo.Data.ID,
		OrderType: orderInfo.Data.Type,
		Price:     price,
		State:     orderInfo.Data.State,
		Symbol:    orderInfo.Data.Symbol,
		Side:      orderInfo.Data.Side,
	}

	err := moi.Create(db)

	if err == nil {
		fmt.Printf("save order,%s\n", moi)
	}
	return err
}
