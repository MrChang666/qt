package service

import (
	"fmt"
	"time"

	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/MrChang666/qt/util"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

type PendService struct {
	symbol          string          //交易对，如btcusdt
	balance         decimal.Decimal //允许使用的金额，比如100
	assetPrecision  int32
	pricePrecision  int32
	fcClient        *client.FCoinClient
	period          int
	bySide          string
	sellOrderResult *client.OrderResult
	buyOrderResult  *client.OrderResult
	midRate         decimal.Decimal
	maxRate         decimal.Decimal
	minRate         decimal.Decimal
	buyPrice        decimal.Decimal
	sellPrice       decimal.Decimal
	minAsset        decimal.Decimal
}

var one = decimal.New(1, 0)

func NewPendService(symbol string, balance, midRate, maxRate, minRate, minAsset decimal.Decimal, assetPrecision, pricepPrecision int32, fcClient *client.FCoinClient, period int, bySide string) *PendService {
	ps := &PendService{
		symbol:         symbol,
		balance:        balance,
		assetPrecision: assetPrecision,
		pricePrecision: pricepPrecision,
		fcClient:       fcClient,
		period:         period,
		bySide:         bySide,
		midRate:        midRate,
		maxRate:        maxRate,
		minRate:        minRate,
		minAsset:       minAsset,
	}
	return ps
}

func (ps *PendService) Run() {

	for {

		time.Sleep(time.Second * time.Duration(ps.period))

		ticker, err := ps.fcClient.GetLatestTickerBySymbol(ps.symbol)
		if err != nil {
			log.Errorf("get ticker info failed,%v", err)
			continue
		}
		curPrice := decimal.NewFromFloat(ticker.Data.Ticker[0]).Round(ps.pricePrecision)

		err = ps.cancelSellOrder(curPrice)
		if err != nil {
			log.Error(err)
			continue
		}

		err = ps.createSellOrder(curPrice)
		if err != nil {
			log.Errorf("create buy order failed,%v", err)
			continue
		}

		err = ps.cancelBuyOrder(curPrice)
		if err != nil {
			log.Error(err)
			continue
		}

		err = ps.createBuyOrder(curPrice)
		if err != nil {
			log.Error(err)
		}

	}
}

/**
 */
func (ps *PendService) createBuyOrder(curPrice decimal.Decimal) error {

	if ps.buyOrderResult != nil {
		return nil
	}

	if ps.bySide == "1" && ps.sellOrderResult != nil {
		return nil
	}

	usdt := util.GetUSDT(ps.symbol)
	if usdt == "" {
		return fmt.Errorf("can't get usdt")
	}

	available, err := ps.fcClient.GetAvailableBalance(usdt)
	if err != nil {
		return fmt.Errorf("get available failed,%v", err)
	}

	if available.LessThan(ps.balance) {
		return nil
	}

	if available.GreaterThan(ps.balance) {
		available = ps.balance
	}

	buyPrice := curPrice.Mul(one.Sub(ps.maxRate)).Round(ps.pricePrecision)

	p := decimal.New(1, ps.assetPrecision)

	assetAmt := available.Div(buyPrice)
	assetAmt = assetAmt.Mul(p).Floor().Div(p)
	//构建买单
	newOrder := &client.NewOrder{
		Amount:    assetAmt.String(),
		OrderType: client.ORDER_TYPE_LIMIT, //限价limit 市价 market
		Exchange:  client.EXCHANGE_MAIN,    //主板
		Side:      client.BUY,              //sell buy
		Symbol:    ps.symbol,
		Price:     buyPrice.String(),
	}
	res, err := ps.fcClient.CreateOrder(newOrder)
	if err != nil {
		return err
	}

	if res.Status != client.ORDER_STATES_SUCCESS {
		log.Errorf("%s,buy order failed,%v", ps.symbol, res)
		return err
	}

	log.Debugf("%s,created buy order,price:%s,amount:%s", ps.symbol, newOrder.Price, newOrder.Amount)

	ps.buyOrderResult = res
	ps.buyPrice = buyPrice
	defer HandlePanic()
	return err
}

func (ps *PendService) createSellOrder(curPrice decimal.Decimal) error {

	if ps.sellOrderResult != nil {
		return nil
	}

	currency := util.GetCurrency(ps.symbol)
	if currency == "" {
		return fmt.Errorf("can't get currency")
	}

	available, err := ps.fcClient.GetAvailableBalance(currency)
	if err != nil {
		return fmt.Errorf("get available failed,%v", err)
	}

	if available.LessThan(ps.minAsset) {
		return nil
	}

	sellPrice := curPrice.Mul(one.Add(ps.midRate)).Round(ps.pricePrecision)

	p := decimal.New(1, ps.assetPrecision)

	assetAmt := available
	assetAmt = assetAmt.Mul(p).Floor().Div(p)

	//构建订单
	newOrder := &client.NewOrder{
		Amount:    assetAmt.String(),
		OrderType: client.ORDER_TYPE_LIMIT, //限价limit 市价 market
		Exchange:  client.EXCHANGE_MAIN,    //主板
		Side:      client.SELL,             //sell buy
		Symbol:    ps.symbol,
		Price:     sellPrice.String(),
	}
	res, err := ps.fcClient.CreateOrder(newOrder)
	if err != nil {
		return err
	}

	if res.Status != client.ORDER_STATES_SUCCESS {
		log.Errorf("%s sell order failed,%v", ps.symbol, res)
		return err
	}
	log.Debugf("%s,created sell order,price:%s,amount:%s", ps.symbol, newOrder.Price, newOrder.Amount)

	ps.sellOrderResult = res
	ps.sellPrice = sellPrice
	defer HandlePanic()
	return err
}

func (ps *PendService) cancelBuyOrder(curPrice decimal.Decimal) error {
	if ps.buyOrderResult == nil {
		return nil
	}

	if ps.buyPrice.GreaterThan(curPrice.Mul(one.Sub(ps.minRate))) || ps.buyPrice.LessThan(curPrice.Mul(one.Sub(ps.maxRate))) {
		log.Debug("begin to cancel buy order")

		res, err := ps.fcClient.CancelOrder(ps.buyOrderResult.Data)
		if err != nil {
			return fmt.Errorf("cancel buy order failed,%v", err)
		}
		if res.Status == client.ORDER_STATES_SUCCESS || res.Status == client.CANCEL_SUCCESS_ORDER {
			ps.buyOrderResult = nil
		} else { //都是非正常情况
			return fmt.Errorf("cancel buy order error,%v", res)
		}
	}
	defer HandlePanic()
	return nil
}

func (ps *PendService) cancelSellOrder(curPrice decimal.Decimal) error {
	if ps.sellOrderResult == nil {
		return nil
	}

	if curPrice.Mul(one.Add(ps.minRate)).GreaterThan(ps.sellPrice) || curPrice.Mul(one.Add(ps.maxRate)).LessThan(ps.sellPrice) {

		log.Debug("begin to cancel sell order")
		res, err := ps.fcClient.CancelOrder(ps.sellOrderResult.Data)
		if err != nil {
			return fmt.Errorf("cancel sell order failed,%v", err)
		}
		if res.Status == client.ORDER_STATES_SUCCESS || res.Status == client.CANCEL_SUCCESS_ORDER {
			ps.sellOrderResult = nil
		} else { //都是非正常情况
			return fmt.Errorf("cancel sell order error,%v", res)
		}
	}

	defer HandlePanic()
	return nil
}
