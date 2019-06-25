package client

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
)

const (
	ASS_Key              = "assKey"
	SECRET_KEY           = "secretKey"
	AVAILABLE            = 1
	FROZEN               = 0
	BASE_URL             = "baseUrl"
	BUY                  = "buy"
	SELL                 = "sell"
	PARTIAL_FILLED       = "partial_filled"
	FILLED               = "filled"
	ORDER_STATES_SUCCESS = 0
	ORDER_STATE_CANCEL   = "canceled"
	ORDER_TYPE_LIMIT     = "limit"
	EXCHANGE_MAIN        = "main"
	CANCEL_SUCCESS_ORDER = 3008 //已经成交的订单，执行cancel返回的错误码
	ORDER_INSUFFICIENT   = 1016 //insufficient balance
)

type FCoinClient struct {
	secretKey string
	assetKey  string
	baseUrl   string
}

func NewFCoinClient(secretKey, assKey, baseUrl string) *FCoinClient {
	return &FCoinClient{secretKey: secretKey, assetKey: assKey, baseUrl: baseUrl}
}

func (f *FCoinClient) sign(url string) (string, string) {
	timeStamp := time.Now().UnixNano() / 1000000
	ts := strconv.FormatInt(timeStamp, 10)
	unsignedUrl := http.MethodGet + url + ts
	firstBase64 := base64.StdEncoding.EncodeToString([]byte(unsignedUrl))
	//hmac ,use sha1
	key := []byte(f.secretKey)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(firstBase64))
	urlBuf := mac.Sum(nil)
	encoded := base64.StdEncoding.EncodeToString(urlBuf)
	return encoded, ts
}

func (f *FCoinClient) getResponse(url string, isPrivate bool) ([]byte, error) {
	encoded, timeStamp := f.sign(url)
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if isPrivate {
		request.Header.Set("FC-ACCESS-KEY", f.assetKey)
		request.Header.Set("FC-ACCESS-TIMESTAMP", timeStamp)
		request.Header.Set("FC-ACCESS-SIGNATURE", encoded)
	}
	resp, err := client.Do(request) //发送请求
	if err != nil {
		return nil, err
	}
	if resp == nil {
		log.Errorf("fcoin response is nil")
		return nil, nil
	}
	defer resp.Body.Close() //关闭resp.Body
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		log.Error("content is empty")
		return nil, nil
	}
	return content, err
}

func (f *FCoinClient) getOpenResponse(url string) ([]byte, error) {
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request) //发送请求
	if err != nil {
		return nil, err
	}
	if resp == nil {
		log.Errorf("fcoin response is nil")
		return nil, nil
	}
	defer resp.Body.Close() //关闭resp.Body
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		log.Error("content is empty")
		return nil, nil
	}
	return content, err
}

func (f *FCoinClient) getPostResponse(url, timeStamp, encoded string, body io.Reader) ([]byte, error) {
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("FC-ACCESS-KEY", f.assetKey)

	request.Header.Set("FC-ACCESS-TIMESTAMP", timeStamp)
	request.Header.Set("FC-ACCESS-SIGNATURE", encoded)
	resp, err := client.Do(request) //发送请求
	if err != nil {
		return nil, err
	}
	if resp == nil {
		log.Errorf("fcoin response is nil")
		return nil, nil
	}
	defer resp.Body.Close() //关闭resp.Body
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		log.Error("content is empty")
		return nil, nil
	}
	return content, err
}

type BalanceInfo struct {
	Status int `json:"status"`
	Data   []struct {
		Currency  string `json:"currency"`
		Available string `json:"available"`
		Frozen    string `json:"frozen"`
		Balance   string `json:"balance"`
	} `json:"data"`
}

func (f *FCoinClient) GetBalance() (*BalanceInfo, error) {
	url := f.baseUrl + "/accounts/balance"
	res, err := f.getResponse(url, true)
	if err != nil {
		return nil, err
	}
	balanceInfo := &BalanceInfo{}
	err = json.Unmarshal(res, balanceInfo)
	return balanceInfo, err
}

func (f *FCoinClient) GetUSDTBalance() (*decimal.Decimal, error) {
	url := f.baseUrl + "/accounts/balance"
	res, err := f.getResponse(url, true)
	if err != nil {
		return nil, err
	}
	balanceInfo := &BalanceInfo{}
	err = json.Unmarshal(res, balanceInfo)
	if err != nil {
		return nil, err
	}
	var availableUSDT decimal.Decimal
	for _, v := range balanceInfo.Data {
		if v.Currency == "usdt" {
			availableUSDT, err = decimal.NewFromString(v.Available)
			break
		}
	}
	return &availableUSDT, err
}

//symbol		交易对
//states		订单状态，多种状态联合查询：submitted,partial_filled,partial_canceled,filled,canceled,中间用逗号隔开
//before		查询某个时间戳之前的订单
//after		查询某个时间戳之后的订单
//limit		每页的订单数量，默认为 20 条，最大100
type Order struct {
	after  string
	before string
	limit  string
	states string
	symbol string
}

type OrderList struct {
	Status int `json:"status"`
	Data   []struct {
		ID            string `json:"id"`
		Symbol        string `json:"symbol"`
		Amount        string `json:"amount"`
		Price         string `json:"price"`
		CreatedAt     int64  `json:"created_at"`
		Type          string `json:"type"`
		Side          string `json:"side"`
		FilledAmount  string `json:"filled_amount"`
		ExecutedValue string `json:"executed_value"`
		FillFees      string `json:"fill_fees"`
		FeesIncome    string `json:"fees_income"`
		Source        string `json:"source"`
		Exchange      string `json:"exchange"`
		State         string `json:"state"`
	} `json:"data"`
}
type OrderInfo struct {
	Status int `json:"status"`
	Data   struct {
		ID            string `json:"id"`
		Symbol        string `json:"symbol"`
		Type          string `json:"type"`
		Side          string `json:"side"`
		Price         string `json:"price"`
		Amount        string `json:"amount"`
		State         string `json:"state"`
		ExecutedValue string `json:"executed_value"`
		FillFees      string `json:"fill_fees"`
		FilledAmount  string `json:"filled_amount"`
		CreatedAt     int    `json:"created_at"`
		Source        string `json:"source"`
	} `json:"data"`
}

/**
获取订单列表
*/
func (f *FCoinClient) GetOrders(order *Order) (*OrderList, error) {
	url := f.baseUrl + "/orders"
	params := fmt.Sprintf("?after=%s&before=%s&limit=%s&states=%s&symbol=%s", order.after, order.before, order.limit, order.states, order.symbol)
	url = url + params
	res, err := f.getResponse(url, true)
	if err != nil {
		return nil, err
	}
	orderList := &OrderList{}
	err = json.Unmarshal(res, orderList)
	return orderList, err
}

type OrderResult struct {
	Status int    `json:"status"`
	Data   string `json:"data"`
	Msg    string `json:"msg"`
}

func (f *FCoinClient) GetOrderById(id string) (*OrderInfo, error) {
	url := f.baseUrl + "/orders/" + id
	res, err := f.getResponse(url, true)
	if err != nil {
		return nil, err
	}
	orderInfo := &OrderInfo{}
	err = json.Unmarshal(res, orderInfo)
	return orderInfo, err
}

//symbol	无	交易对
//side	无	交易方向
//type	无	订单类型
//price	无	价格
//amount	无	下单量
//exchange	无	交易区
//account_type	无	账户类型(币币交易不需要填写，杠杆交易：margin)
type NewOrder struct {
	Amount      string `json:"amount"`
	AccountType string `json:"account_ype"`
	Exchange    string `json:"exchange"`
	Side        string `json:"side"`
	Symbol      string `json:"symbol"`
	OrderType   string `json:"type"`
	Price       string `json:"price"`
}

func (f *FCoinClient) CreateOrder(newOrder *NewOrder) (*OrderResult, error) {
	url := f.baseUrl + "/orders"
	formatStr := "account_ype=%s&amount=%s&exchange=%s&price=%s&side=%s&symbol=%s&type=%s"
	params := fmt.Sprintf(formatStr, newOrder.AccountType, newOrder.Amount, newOrder.Exchange, newOrder.Price, newOrder.Side, newOrder.Symbol, newOrder.OrderType)
	timeStamp := time.Now().UnixNano() / 1000000
	ts := strconv.FormatInt(timeStamp, 10)
	//http method+uri+timstamp+params
	unsignedUrl := http.MethodPost + url + ts + params
	firstBase64 := base64.StdEncoding.EncodeToString([]byte(unsignedUrl))
	//hmac ,use sha1
	key := []byte(f.secretKey)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(firstBase64))
	urlBuf := mac.Sum(nil)
	encoded := base64.StdEncoding.EncodeToString(urlBuf)
	b, _ := json.Marshal(newOrder)
	res, err := f.getPostResponse(url, ts, encoded, bytes.NewBuffer(b))
	if err != nil {
		log.Errorf("get orders info failed,%v", err)
		return nil, err
	}
	result := &OrderResult{}
	err = json.Unmarshal(res, result)
	return result, err

}

/**
"最新成交价",
 "最近一笔成交的成交量",
 "最大买一价",
 "最大买一量",
 "最小卖一价",
 "最小卖一量",
 "24小时前成交价",
 "24小时内最高价",
 "24小时内最低价",
 "24小时内基准货币成交量, 如 btcusdt 中 btc 的量",
 "24小时内计价货币成交量, 如 btcusdt 中 usdt 的量"
*/
type TickerInfo struct {
	Status int `json:"status"`
	Data   struct {
		Type   string    `json:"type"`
		Seq    int       `json:"seq"`
		Ticker []float64 `json:"ticker"`
	} `json:"data"`
}

/**
symbol :btcusdt
*/
func (f *FCoinClient) GetLatestTickerBySymbol(symbol string) (*TickerInfo, error) {
	url := f.baseUrl + "/market/ticker/" + symbol
	body, err := f.getOpenResponse(url)
	if err != nil {
		return nil, err
	}
	ticker := &TickerInfo{}
	err = json.Unmarshal(body, ticker)
	return ticker, err
}

type CancelResult struct {
	Status int `json:"status"`
	Data   []struct {
		Price        string `json:"price"`
		FillFees     string `json:"fill_fees"`
		FilledAmount string `json:"filled_amount"`
		Side         string `json:"side"`
		Type         string `json:"type"`
		CreatedAt    int    `json:"created_at"`
	} `json:"data"`
}

func (f *FCoinClient) CancelOrder(id string) (*CancelResult, error) {
	url := f.baseUrl + "/orders/" + id + "/submit-cancel"
	timeStamp := time.Now().UnixNano() / 1000000
	ts := strconv.FormatInt(timeStamp, 10)
	unsignedUrl := http.MethodPost + url + ts
	firstBase64 := base64.StdEncoding.EncodeToString([]byte(unsignedUrl))
	//hmac ,use sha1
	key := []byte(f.secretKey)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(firstBase64))
	urlBuf := mac.Sum(nil)
	encoded := base64.StdEncoding.EncodeToString(urlBuf)
	res, err := f.getPostResponse(url, ts, encoded, nil)

	if err != nil {
		return nil, err
	}
	result := &CancelResult{}
	err = json.Unmarshal(res, result)
	return result, err
}

type Candle struct {
	Status int `json:"status"`
	Data   []struct {
		Open     float64 `json:"open"`
		Close    float64 `json:"close"`
		High     float64 `json:"high"`
		QuoteVol float64 `json:"quote_vol"`
		ID       int     `json:"id"`
		Count    int     `json:"count"`
		Low      float64 `json:"low"`
		Seq      int64   `json:"seq"`
		BaseVol  float64 `json:"base_vol"`
	} `json:"data"`
}

//返回的数组顺序是从[0]是当下的，数据从最新往前排
func (f *FCoinClient) GetCandle(symbol, resolution, limit string) (*Candle, error) {
	if limit == "" {
		limit = "21"
	}
	url := f.baseUrl + "/market/candles/" + resolution + "/" + symbol + "?limit=" + limit
	content, err := f.getOpenResponse(url)
	if err != nil {
		return nil, err
	}
	c := &Candle{}
	err = json.Unmarshal(content, c)
	return c, err
}

type Depth struct {
	Status int `json:"status"`
	Data   struct {
		Bids []float64 `json:"bids"`
		Asks []float64 `json:"asks"`
		Ts   int64     `json:"ts"`
		Seq  int64     `json:"seq"`
		Type string    `json:"type"`
	} `json:"data"`
}

func (f *FCoinClient) GetDepth(symbol, depth string) (*Depth, error) {
	url := f.baseUrl + "/market/depth/" + depth + "/" + symbol
	content, err := f.getOpenResponse(url)
	if err != nil {
		return nil, err
	}
	d := &Depth{}
	err = json.Unmarshal(content, d)
	return d, err
}

func (f *FCoinClient) GetAvailableBalance(currency string) (decimal.Decimal, error) {
	var bal decimal.Decimal
	bals, err := f.GetBalance()

	if err != nil {
		return bal, err
	}

	for _, v := range bals.Data {
		if v.Currency == currency {
			bal, err = decimal.NewFromString(v.Available)
			break
		}
	}

	return bal, err
}

func (f *FCoinClient) GetFrozenBalance(currency string) (decimal.Decimal, error) {
	var bal decimal.Decimal
	bals, err := f.GetBalance()

	if err != nil {
		return bal, err
	}

	for _, v := range bals.Data {
		if v.Currency == currency {
			bal, err = decimal.NewFromString(v.Frozen)
			break
		}
	}

	return bal, err
}
