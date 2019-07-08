package service

import (
	"encoding/json"
	"fmt"
	"github.com/FCoinCommunity/fcoin-go-sdk/fcoin"
	"github.com/MrChang666/fcoin-api-go/client"
	"github.com/MrChang666/qt/model"
	"github.com/MrChang666/qt/util"
	"github.com/jinzhu/gorm"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type DigService struct {
	symbol          string          //交易对，如btcusdt
	balance         decimal.Decimal //允许使用的金额，比如100
	assetPrecision  int32
	pricePrecision  int32
	fcClient        *client.FCoinClient
	buyOrderResult  *client.OrderResult
	sellOrderResult *client.OrderResult
	minBalance      decimal.Decimal
	minAsset        decimal.Decimal
	buyLevel        int
	sellLevel       int
	period          int
	bySide          string
	orderChan       chan string
	db              *gorm.DB
	depthType       string
}

type WsDepth struct {
	Bids []float64 `json:"bids"`
	Asks []float64 `json:"asks"`
	Ts   int64     `json:"ts"`
	Seq  int       `json:"seq"`
	Type string    `json:"type"`
}

func NewDigService(symbol string, balance, minBalance, minAsset decimal.Decimal, assetPrecision, pricepPrecision int32, fcClient *client.FCoinClient, buyLevel, sellLevel, period int, bySide string, db *gorm.DB, depthType string) *DigService {
	ds := &DigService{
		symbol:         symbol,
		balance:        balance,
		assetPrecision: assetPrecision,
		pricePrecision: pricepPrecision,
		fcClient:       fcClient,
		minBalance:     minBalance,
		minAsset:       minAsset,
		buyLevel:       buyLevel,
		sellLevel:      sellLevel,
		period:         period,
		bySide:         bySide,
		orderChan:      make(chan string, 128),
		db:             db,
		depthType:      depthType,
	}
	return ds
}

func handlePanic() {
	if err := recover(); err != nil {
		log.Error("panic err:", err)
	}
}

func (ds *DigService) Run() {

	api := fcoin.Client{}
	if err := api.InitWS(); err != nil {
		log.Fatal(err)
	}

	if err := api.WSSubscribe("", ds.depthType); err != nil {
		log.Fatal(err)
	}

	heartBeatingTime := time.Now()

	for {

		time.Sleep(time.Second * time.Duration(ds.period))

		err := ds.cancelBuyOrder()
		if err != nil {
			log.Error(err)
		}

		err = ds.cancelSellOrder()
		if err != nil {
			log.Error(err)
			continue
		}

		if time.Now().Sub(heartBeatingTime) > time.Second*29 {

			resp, err := api.WSPing()
			if err != nil {
				log.Error(err)
				continue
			}
			heartBeatingTime = time.Now()
			log.Infof("heart beating,%v", resp)
		}

		_, rsp, err := api.WS.ReadMessage()
		if err != nil {
			log.Errorf("ws ReadMessage failed,%v", err)
			continue
		}

		//ping 返回的结果
		if strings.Contains(string(rsp), "topics") {
			continue
		}

		depth := &WsDepth{}
		err = json.Unmarshal(rsp, depth)
		if err != nil {
			log.Error(err)
			continue
		}

		if depth == nil || len(depth.Asks) < 30 || len(depth.Bids) < 30 {
			log.Error("depth data is not enough")
			continue
		}

		time.Sleep(time.Millisecond * 101)
		//创建卖单
		err = ds.createSellOrder(depth)
		if err != nil {
			log.Errorf("create buy order failed,%v", err)
			continue
		}

		time.Sleep(time.Millisecond * 101)
		//创建买单
		err = ds.createBuyOrder(depth)
		if err != nil {
			log.Errorf("create buy order failed,%v", err)
		}

	}
}

/**
 */
func (ds *DigService) createBuyOrder(depth *WsDepth) error {

	if ds.buyOrderResult != nil {
		return nil
	}

	if ds.bySide == "1" && ds.sellOrderResult != nil {
		log.Infof("%s,one side trade", ds.symbol)
		return nil
	}

	usdt := util.GetUSDT(ds.symbol)
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

	buyPrice := decimal.NewFromFloat(depth.Bids[(ds.buyLevel-1)*2])

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

	log.Debugf("%s,created buy order,price:%s,amount:%s", ds.symbol, newOrder.Price, newOrder.Amount)

	ds.buyOrderResult = res
	defer handlePanic()
	return err
}

func (ds *DigService) createSellOrder(depth *WsDepth) error {

	if ds.sellOrderResult != nil {
		return nil
	}

	currency := util.GetCurrency(ds.symbol)
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

	sellPrice := decimal.NewFromFloat(depth.Asks[(ds.sellLevel-1)*2])

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

	log.Debugf("%s,created sell order,price:%s,amount:%s", ds.symbol, newOrder.Price, newOrder.Amount)

	ds.sellOrderResult = res
	defer handlePanic()
	return err
}

func (ds *DigService) cancelBuyOrder() error {
	if ds.buyOrderResult == nil {
		return nil
	}
	log.Debug("begin to cancel buy order")
	res, err := ds.fcClient.CancelOrder(ds.buyOrderResult.Data)
	if err != nil {
		return fmt.Errorf("cancel buy order failed,%v", err)
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
		return fmt.Errorf("cancel buy order error,%v", res)
	}

	defer handlePanic()
	return nil
}

func (ds *DigService) cancelSellOrder() error {
	if ds.sellOrderResult == nil {
		return nil
	}
	log.Debug("begin to cancel sell order")
	res, err := ds.fcClient.CancelOrder(ds.sellOrderResult.Data)
	if err != nil {
		return fmt.Errorf("cancel sell order failed,%v", err)
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
		return fmt.Errorf("cancel sell order error,%v", res)
	}
	defer handlePanic()
	return nil
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
