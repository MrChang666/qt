package model

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/shopspring/decimal"
)

func InitDB(db *gorm.DB) {
	db.AutoMigrate(&OrderInfo{})
}

//"id": "9d17a03b852e48c0b3920c7412867623",
//"symbol": "string",
//"type": "limit",
//"side": "buy",
//"price": "string",
//"amount": "string",
//"state": "submitted",
//"executed_value": "string",
//"fill_fees": "string",
//"filled_amount": "string",
//"created_at": 0,
//"source": "web"

type OrderInfo struct {
	gorm.Model
	OrderId   string          `gorm:"size:64" json:"order_id"`
	OrderType string          `gorm:"size:8" json:"type"`
	Price     decimal.Decimal `gorm:"type:decimal(32,20);default:0" json:"price"`
	State     string          `gorm:"size:16;" json:"state"`
	Amount    decimal.Decimal `gorm:"type:decimal(32,20);default:0" json:"amount"`
	Symbol    string          `gorm:"size:8;index" json:"symbol"`
	Side      string          `gorm:"size:8;" json:"side"`
}

func (oi OrderInfo) String() string {
	return fmt.Sprintf("symbol:%s,price:%s,amount:%s,side:%s", oi.Symbol, oi.Price, oi.Amount, oi.Side)
}

func (*OrderInfo) TableName() string { return "order_info" }

func (order *OrderInfo) Create(db *gorm.DB) error {
	return db.Create(order).Error
}
