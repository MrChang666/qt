package util

import (
	"strings"
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
