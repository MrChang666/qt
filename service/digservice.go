package service

import (
	"fmt"
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/MrChang666/qt/model"
	"github.com/jinzhu/gorm"
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
	period          int
	bySide          string
	orderChan       chan string
	db              *gorm.DB
	diffBuyRate     decimal.Decimal
	diffSellRate    decimal.Decimal
}

func NewDigService(symbol string, balance, minBalance, minAsset, diffBuyRate, diffSellRate decimal.Decimal, assetPrecision, pricepPrecision int32, fcClient *client.FCoinClient, period int, bySide string, db *gorm.DB) *DigService {
	ds := &DigService{
		symbol:          symbol,
		balance:         balance,
		assetPrecision:  assetPrecision,
		pricepPrecision: pricepPrecision,
		fcClient:        fcClient,
		minBalance:      minBalance,
		minAsset:        minAsset,
		period:          period,
		bySide:          bySide,
		orderChan:       make(chan string, 256),
		db:              db,
		diffBuyRate:     diffBuyRate,
		diffSellRate:    diffSellRate,
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

		ticker, err := ds.fcClient.GetLatestTickerBySymbol(ds.symbol)
		if len(ticker.Data.Ticker) == 0 {
			log.Error("ticker info is nil")
			continue
		}

		curPrice := decimal.NewFromFloat(ticker.Data.Ticker[0])
		if curPrice.LessThanOrEqual(decimal.Zero) {
			log.Error("curprice is zero")
			continue
		}
		//创建卖单
		err = ds.createSellOrder(curPrice)
		if err != nil {
			log.Errorf("create buy order failed,%v", err)
			continue
		}

		//创建买单
		err = ds.createBuyOrder(curPrice)
		if err != nil {
			log.Errorf("create buy order failed,%v", err)
		}

		time.Sleep(time.Second * time.Duration(ds.period))
	}
}

func (ds *DigService) createBuyOrder(curPrice decimal.Decimal) error {

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

	one := decimal.New(1, 0)

	buyPrice := curPrice.Mul(one.Sub(ds.diffBuyRate))
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

func (ds *DigService) createSellOrder(curPrice decimal.Decimal) error {

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

	one := decimal.New(1, 0)
	sellPrice := curPrice.Mul(one.Add(ds.diffSellRate))

	assetAmt := ds.balance.Div(sellPrice)

	if assetAmt.GreaterThan(available) {
		assetAmt = available
	}

	p := decimal.New(1, ds.assetPrecision)
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
		select {
		case ds.orderChan <- ds.buyOrderResult.Data:
		}

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
		select {
		case ds.orderChan <- ds.sellOrderResult.Data:
		}

		ds.sellOrderResult = nil
	} else { //都是非正常情况
		log.Errorf("cancel sell order error,%v", res)
	}
	defer handlePanic()
}

//btcusdt  btcpax btctusd

func getUSDT(symbol string) string {
	var usdt string
	if strings.HasSuffix(symbol, USDT) {
		usdt = USDT
	} else if strings.HasSuffix(symbol, PAX) {
		usdt = PAX
	}
	return usdt
}

func getCurrency(symbol string) string {
	var cur string
	if strings.HasSuffix(symbol, USDT) {
		cur = strings.TrimSuffix(symbol, USDT)
	} else if strings.HasSuffix(symbol, PAX) {
		cur = strings.TrimSuffix(symbol, PAX)
	}
	return cur
}

func (ds *DigService) SaveOrder() {
	for {
		select {
		case orderId := <-ds.orderChan:
			orderInfo, err := ds.fcClient.GetOrderById(orderId)
			if err != nil {
				log.Errorf("check success order failed.%v", err)
			}
			amount, _ := decimal.NewFromString(orderInfo.Data.Amount)
			price, _ := decimal.NewFromString(orderInfo.Data.Price)
			moi := &model.OrderInfo{
				Amount:    amount,
				OrderId:   orderId,
				OrderType: orderInfo.Data.Type,
				Price:     price,
				State:     orderInfo.Data.State,
				Symbol:    orderInfo.Data.Symbol,
				Side:      orderInfo.Data.Side,
			}

			err = moi.Create(ds.db)
			if err != nil {
				log.Errorf("save order %s failed,%v", orderId, err)
			}

			log.Debugf("save order,%s", moi)
		}
		time.Sleep(time.Second * 5)
	}
}
