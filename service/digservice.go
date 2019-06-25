package service

import (
	"fmt"
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

const (
	USDT = "usdt"
	PAX  = "pax"
)

type DigService struct {
	symbol          string          //交易对，如btcusdt
	balance         decimal.Decimal //允许使用的金额，比如100
	assetPrecision  int32
	pricepPrecision int32
	fcClient        *client.FCoinClient
	buyOrderResult  *client.OrderResult
	sellOrderResult *client.OrderResult
	minBalance      decimal.Decimal
	minAsset        decimal.Decimal
	buyLevel        int
	sellLevel       int
	period          int
	bySide          string
}

func NewDigService(symbol string, balance, minBalance, minAsset decimal.Decimal, assetPrecision, pricepPrecision int32, fcClient *client.FCoinClient, sellLevel, buyLevel, period int, bySide string) *DigService {
	ds := &DigService{
		symbol:          symbol,
		balance:         balance,
		assetPrecision:  assetPrecision,
		pricepPrecision: pricepPrecision,
		fcClient:        fcClient,
		minBalance:      minBalance,
		minAsset:        minAsset,
		buyLevel:        buyLevel,
		sellLevel:       sellLevel,
		period:          period,
		bySide:          bySide,
	}
	return ds
}

func handlePanic() {
	if err := recover(); err != nil {
		log.Error("panic err:", err)
	}
}

func (ds *DigService) Run() {
	for {

		ds.cancelBuyOrder()
		ds.cancelSellOrder()
		depth, err := ds.fcClient.GetDepth(ds.symbol, "L20")
		if err != nil {
			log.Error(err)
			continue
		}

		if depth == nil || len(depth.Data.Asks) < 30 || len(depth.Data.Bids) < 30 {
			log.Error("depth data is not enough")
			return
		}

		//创建卖单
		err = ds.createSellOrder(depth)
		if err != nil {
			log.Errorf("create buy order failed,%v", err)
		}

		//创建买单
		err = ds.createBuyOrder(depth)
		if err != nil {
			log.Errorf("create buy order failed,%v", err)
		}

		time.Sleep(time.Second * time.Duration(ds.period))
	}
}

/**
1、创建6-15之间的买单 12
*/
func (ds *DigService) createBuyOrder(depth *client.Depth) error {

	if ds.buyOrderResult != nil {
		return nil
	}

	if ds.bySide == "1" && ds.sellOrderResult != nil {
		log.Infof("%s,one side trade", ds.symbol)
		return nil
	}

	usdt := getUSDT(ds.symbol)
	if usdt == "" {
		return fmt.Errorf("can't get usdt")
	}

	available, err := ds.fcClient.GetAvailableBalance(usdt)
	if err != nil {
		return fmt.Errorf("get available failed,%v", err)
	}

	//如果available小于minBalance，直接返回
	if available.LessThan(ds.minBalance) {
		return nil
	}

	if available.GreaterThan(ds.balance) {
		available = ds.balance
	}

	log.Debugf("%s,begin to create buy order", ds.symbol)
	buyPrice := decimal.NewFromFloat(depth.Data.Bids[(ds.buyLevel-1)*2])

	p := decimal.New(1, ds.assetPrecision)

	assetAmt := available.Div(buyPrice)
	assetAmt = assetAmt.Mul(p).Floor().Div(p)
	//构建买单
	newOrder := &client.NewOrder{
		Amount:    assetAmt.String(),
		OrderType: client.ORDER_TYPE_LIMIT, //限价limit 市价 market
		Exchange:  client.EXCHANGE_MAIN,    //主板
		Side:      client.BUY,              //sell buy
		Symbol:    ds.symbol,
		Price:     buyPrice.String(),
	}
	res, err := ds.fcClient.CreateOrder(newOrder)
	if err != nil {
		return err
	}

	if res.Status != client.ORDER_STATES_SUCCESS {
		log.Errorf("%s,buy order failed,%v", ds.symbol, res)
		return err
	}

	ds.buyOrderResult = res
	defer handlePanic()
	return err
}

func (ds *DigService) createSellOrder(depth *client.Depth) error {

	if ds.sellOrderResult != nil {
		return nil
	}

	currency := getCurrency(ds.symbol)
	if currency == "" {
		return fmt.Errorf("can't get currency")
	}

	available, err := ds.fcClient.GetAvailableBalance(currency)
	if err != nil {
		return fmt.Errorf("get available failed,%v", err)
	}

	//如果available小于minAsset，直接返回
	if available.LessThan(ds.minAsset) {
		return nil
	}

	log.Debugf("%s,begin to create sell order", ds.symbol)
	sellPrice := decimal.NewFromFloat(depth.Data.Asks[(ds.sellLevel-1)*2])

	p := decimal.New(1, ds.assetPrecision)

	assetAmt := available
	assetAmt = assetAmt.Mul(p).Floor().Div(p)
	//构建订单
	newOrder := &client.NewOrder{
		Amount:    assetAmt.String(),
		OrderType: client.ORDER_TYPE_LIMIT, //限价limit 市价 market
		Exchange:  client.EXCHANGE_MAIN,    //主板
		Side:      client.SELL,             //sell buy
		Symbol:    ds.symbol,
		Price:     sellPrice.String(),
	}
	res, err := ds.fcClient.CreateOrder(newOrder)
	if err != nil {
		return err
	}

	if res.Status != client.ORDER_STATES_SUCCESS {
		log.Errorf("%s sell order failed,%v", ds.symbol, res)
		return err
	}

	ds.sellOrderResult = res
	defer handlePanic()
	return err
}

func (ds *DigService) cancelBuyOrder() {
	if ds.buyOrderResult == nil {
		return
	}
	log.Debug("begin to cancel buy order")
	res, err := ds.fcClient.CancelOrder(ds.buyOrderResult.Data)
	if err != nil {
		log.Errorf("cancel buy order failed,%v", err)
		return
	}
	if res.Status == client.ORDER_STATES_SUCCESS {
		ds.buyOrderResult = nil
	} else if res.Status == client.CANCEL_SUCCESS_ORDER {
		//记录成交的情况
		orderInfo, err := ds.fcClient.GetOrderById(ds.buyOrderResult.Data)

		if err != nil {
			log.Errorf("get order info failed,%v", err)
		}

		log.Infof("side:%s,symbol:%s,price:%s,amount:%s", orderInfo.Data.Side, orderInfo.Data.Symbol, orderInfo.Data.Price, orderInfo.Data.Amount)

		ds.buyOrderResult = nil
	} else { //都是非正常情况
		log.Errorf("cancel buy order error,%v", res)
	}

	defer handlePanic()
}

func (ds *DigService) cancelSellOrder() {
	if ds.sellOrderResult == nil {
		return
	}
	log.Debug("begin to cancel sell order")
	res, err := ds.fcClient.CancelOrder(ds.sellOrderResult.Data)
	if err != nil {
		log.Errorf("cancel sell order failed,%v", err)
		return
	}
	if res.Status == client.ORDER_STATES_SUCCESS {
		ds.sellOrderResult = nil
	} else if res.Status == client.CANCEL_SUCCESS_ORDER {
		//记录成交的情况
		orderInfo, err := ds.fcClient.GetOrderById(ds.sellOrderResult.Data)

		if err != nil {
			log.Errorf("get sell order info failed,%v", err)
		}

		log.Infof("side:%s,symbol:%s,price:%s,amount:%s", orderInfo.Data.Side, orderInfo.Data.Symbol, orderInfo.Data.Price, orderInfo.Data.Amount)

		ds.sellOrderResult = nil
	} else { //都是非正常情况
		log.Errorf("cancel sell order error,%v", res)
	}
	defer handlePanic()
}

//btcusdt  btcpax btctusd

func getUSDT(symbol string) string {
	if strings.HasSuffix(symbol, USDT) {
		return USDT
	}
	if strings.HasSuffix(symbol, PAX) {
		return PAX
	}
	return ""
}

func getCurrency(symbol string) string {
	if strings.HasSuffix(symbol, USDT) {
		return strings.TrimSuffix(symbol, USDT)
	}
	if strings.HasSuffix(symbol, PAX) {
		return strings.TrimSuffix(symbol, PAX)
	}
	return ""
}
