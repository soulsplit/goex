package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	. "github.com/soulsplit/goex"
)

const (
	GLOBAL_API_BASE_URL = "https://api.binance.com"
	US_API_BASE_URL     = "https://api.binance.us"
	JE_API_BASE_URL     = "https://api.binance.je"
	//API_V1       = API_BASE_URL + "api/v1/"
	//API_V3       = API_BASE_URL + "api/v3/"

	TICKER_URI             = "ticker/24hr?symbol=%s"
	TICKERS_URI            = "ticker/allBookTickers"
	DEPTH_URI              = "depth?symbol=%s&limit=%d"
	ACCOUNT_URI            = "account?"
	ORDER_URI              = "order"
	UNFINISHED_ORDERS_INFO = "openOrders?"
	KLINE_URI              = "klines"
	SERVER_TIME_URL        = "time"
)

var _INERNAL_KLINE_PERIOD_CONVERTER = map[KlinePeriod]string{
	KLINE_PERIOD_1MIN:   "1m",
	KLINE_PERIOD_3MIN:   "3m",
	KLINE_PERIOD_5MIN:   "5m",
	KLINE_PERIOD_15MIN:  "15m",
	KLINE_PERIOD_30MIN:  "30m",
	KLINE_PERIOD_60MIN:  "1h",
	KLINE_PERIOD_1H:     "1h",
	KLINE_PERIOD_2H:     "2h",
	KLINE_PERIOD_4H:     "4h",
	KLINE_PERIOD_6H:     "6h",
	KLINE_PERIOD_8H:     "8h",
	KLINE_PERIOD_12H:    "12h",
	KLINE_PERIOD_1DAY:   "1d",
	KLINE_PERIOD_3DAY:   "3d",
	KLINE_PERIOD_1WEEK:  "1w",
	KLINE_PERIOD_1MONTH: "1M",
}

type Filter struct {
	FilterType          string  `json:"filterType"`
	MaxPrice            float64 `json:"maxPrice,string"`
	MinPrice            float64 `json:"minPrice,string"`
	TickSize            float64 `json:"tickSize,string"`
	MultiplierUp        float64 `json:"multiplierUp,string"`
	MultiplierDown      float64 `json:"multiplierDown,string"`
	AvgPriceMins        int     `json:"avgPriceMins"`
	MinQty              float64 `json:"minQty,string"`
	MaxQty              float64 `json:"maxQty,string"`
	StepSize            float64 `json:"stepSize,string"`
	MinNotional         float64 `json:"minNotional,string"`
	ApplyToMarket       bool    `json:"applyToMarket"`
	Limit               int     `json:"limit"`
	MaxNumAlgoOrders    int     `json:"maxNumAlgoOrders"`
	MaxNumIcebergOrders int     `json:"maxNumIcebergOrders"`
	MaxNumOrders        int     `json:"maxNumOrders"`
}

type RateLimit struct {
	Interval      string `json:"interval"`
	IntervalNum   int64  `json:"intervalNum"`
	Limit         int64  `json:"limit"`
	RateLimitType string `json:"rateLimitType"`
}

type TradeSymbol struct {
	Symbol                     string   `json:"symbol"`
	Status                     string   `json:"status"`
	BaseAsset                  string   `json:"baseAsset"`
	BaseAssetPrecision         int      `json:"baseAssetPrecision"`
	QuoteAsset                 string   `json:"quoteAsset"`
	QuotePrecision             int      `json:"quotePrecision"`
	BaseCommissionPrecision    int      `json:"baseCommissionPrecision"`
	QuoteCommissionPrecision   int      `json:"quoteCommissionPrecision"`
	Filters                    []Filter `json:"filters"`
	IcebergAllowed             bool     `json:"icebergAllowed"`
	IsMarginTradingAllowed     bool     `json:"isMarginTradingAllowed"`
	IsSpotTradingAllowed       bool     `json:"isSpotTradingAllowed"`
	OcoAllowed                 bool     `json:"ocoAllowed"`
	QuoteOrderQtyMarketAllowed bool     `json:"quoteOrderQtyMarketAllowed"`
	OrderTypes                 []string `json:"orderTypes"`
}

func (ts TradeSymbol) GetMinAmount() float64 {
	for _, v := range ts.Filters {
		if v.FilterType == "LOT_SIZE" {
			return v.MinQty
		}
	}
	return 0
}

func (ts TradeSymbol) GetAmountPrecision() int {
	for _, v := range ts.Filters {
		if v.FilterType == "LOT_SIZE" {
			step := strconv.FormatFloat(v.StepSize, 'f', -1, 64)
			pres := strings.Split(step, ".")
			if len(pres) == 1 {
				return 0
			}
			return len(pres[1])
		}
	}
	return 0
}

func (ts TradeSymbol) GetMinPrice() float64 {
	for _, v := range ts.Filters {
		if v.FilterType == "PRICE_FILTER" {
			return v.MinPrice
		}
	}
	return 0
}

func (ts TradeSymbol) GetMinValue() float64 {
	for _, v := range ts.Filters {
		if v.FilterType == "MIN_NOTIONAL" {
			return v.MinNotional
		}
	}
	return 0
}

func (ts TradeSymbol) GetPricePrecision() int {
	for _, v := range ts.Filters {
		if v.FilterType == "PRICE_FILTER" {
			step := strconv.FormatFloat(v.TickSize, 'f', -1, 64)
			pres := strings.Split(step, ".")
			if len(pres) == 1 {
				return 0
			}
			return len(pres[1])
		}
	}
	return 0
}

type ExchangeInfo struct {
	Timezone        string        `json:"timezone"`
	ServerTime      int           `json:"serverTime"`
	ExchangeFilters []interface{} `json:"exchangeFilters,omitempty"`
	RateLimits      []RateLimit   `json:"rateLimits"`
	Symbols         []TradeSymbol `json:"symbols"`
}

type Exchange struct {
	accessKey  string
	secretKey  string
	baseUrl    string
	apiV1      string
	apiV3      string
	httpClient *http.Client
	timeOffset int64 //nanosecond
	*ExchangeInfo
}

func (exchange *Exchange) buildParamsSigned(postForm *url.Values) error {
	postForm.Set("recvWindow", "60000")
	tonce := strconv.FormatInt(time.Now().UnixNano()+exchange.timeOffset, 10)[0:13]
	postForm.Set("timestamp", tonce)
	payload := postForm.Encode()
	sign, _ := GetParamHmacSHA256Sign(exchange.secretKey, payload)
	postForm.Set("signature", sign)
	return nil
}

func New(client *http.Client, api_key, secret_key string) *Exchange {
	return NewWithConfig(&APIConfig{
		HttpClient:   client,
		Endpoint:     GLOBAL_API_BASE_URL,
		ApiKey:       api_key,
		ApiSecretKey: secret_key})
}

func NewWithConfig(config *APIConfig) *Exchange {
	if config.Endpoint == "" {
		config.Endpoint = GLOBAL_API_BASE_URL
	}

	bn := &Exchange{
		baseUrl:    config.Endpoint,
		apiV1:      config.Endpoint + "/api/v1/",
		apiV3:      config.Endpoint + "/api/v3/",
		accessKey:  config.ApiKey,
		secretKey:  config.ApiSecretKey,
		httpClient: config.HttpClient}
	bn.setTimeOffset()
	return bn
}

func (exchange *Exchange) GetExchangeName() string {
	return BINANCE
}

func (exchange *Exchange) Ping() bool {
	_, err := HttpGet(exchange.httpClient, exchange.apiV3+"ping")
	if err != nil {
		return false
	}
	return true
}

func (exchange *Exchange) setTimeOffset() error {
	respmap, err := HttpGet(exchange.httpClient, exchange.apiV3+SERVER_TIME_URL)
	if err != nil {
		return err
	}

	stime := int64(ToInt(respmap["serverTime"]))
	st := time.Unix(stime/1000, 1000000*(stime%1000))
	lt := time.Now()
	offset := st.Sub(lt).Nanoseconds()
	exchange.timeOffset = int64(offset)
	return nil
}

func (exchange *Exchange) GetTicker(currency CurrencyPair) (*Ticker, error) {
	tickerUri := exchange.apiV3 + fmt.Sprintf(TICKER_URI, currency.ToSymbol(""))
	tickerMap, err := HttpGet(exchange.httpClient, tickerUri)

	if err != nil {
		return nil, err
	}

	var ticker Ticker
	ticker.Pair = currency
	t, _ := tickerMap["closeTime"].(float64)
	ticker.Date = uint64(t / 1000)
	ticker.Last = ToFloat64(tickerMap["lastPrice"])
	ticker.Buy = ToFloat64(tickerMap["bidPrice"])
	ticker.Sell = ToFloat64(tickerMap["askPrice"])
	ticker.Low = ToFloat64(tickerMap["lowPrice"])
	ticker.High = ToFloat64(tickerMap["highPrice"])
	ticker.Vol = ToFloat64(tickerMap["volume"])
	return &ticker, nil
}

func (exchange *Exchange) GetDepth(size int, currencyPair CurrencyPair) (*Depth, error) {
	if size <= 5 {
		size = 5
	} else if size <= 10 {
		size = 10
	} else if size <= 20 {
		size = 20
	} else if size <= 50 {
		size = 50
	} else if size <= 100 {
		size = 100
	} else if size <= 500 {
		size = 500
	} else {
		size = 1000
	}

	apiUrl := fmt.Sprintf(exchange.apiV3+DEPTH_URI, currencyPair.ToSymbol(""), size)
	resp, err := HttpGet(exchange.httpClient, apiUrl)
	if err != nil {
		return nil, err
	}

	if _, isok := resp["code"]; isok {
		return nil, errors.New(resp["msg"].(string))
	}

	bids := resp["bids"].([]interface{})
	asks := resp["asks"].([]interface{})

	depth := new(Depth)
	depth.Pair = currencyPair
	depth.UTime = time.Now()
	n := 0
	for _, bid := range bids {
		_bid := bid.([]interface{})
		amount := ToFloat64(_bid[1])
		price := ToFloat64(_bid[0])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.BidList = append(depth.BidList, dr)
		n++
		if n == size {
			break
		}
	}

	n = 0
	for _, ask := range asks {
		_ask := ask.([]interface{})
		amount := ToFloat64(_ask[1])
		price := ToFloat64(_ask[0])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.AskList = append(depth.AskList, dr)
		n++
		if n == size {
			break
		}
	}

	sort.Sort(sort.Reverse(depth.AskList))

	return depth, nil
}

func (exchange *Exchange) placeOrder(amount, price string, pair CurrencyPair, orderType, orderSide string) (*Order, error) {
	path := exchange.apiV3 + ORDER_URI
	params := url.Values{}
	params.Set("symbol", pair.ToSymbol(""))
	params.Set("side", orderSide)
	params.Set("type", orderType)
	params.Set("newOrderRespType", "ACK")
	params.Set("quantity", amount)

	switch orderType {
	case "LIMIT":
		params.Set("timeInForce", "GTC")
		params.Set("price", price)
	case "MARKET":
		params.Set("newOrderRespType", "RESULT")
	}

	exchange.buildParamsSigned(&params)

	resp, err := HttpPostForm2(exchange.httpClient, path, params,
		map[string]string{"X-MBX-APIKEY": exchange.accessKey})
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	orderId := ToInt(respmap["orderId"])
	if orderId <= 0 {
		return nil, errors.New(string(resp))
	}

	side := BUY
	if orderSide == "SELL" {
		side = SELL
	}

	dealAmount := ToFloat64(respmap["executedQty"])
	cummulativeQuoteQty := ToFloat64(respmap["cummulativeQuoteQty"])
	avgPrice := 0.0
	if cummulativeQuoteQty > 0 && dealAmount > 0 {
		avgPrice = cummulativeQuoteQty / dealAmount
	}

	return &Order{
		Currency:   pair,
		OrderID:    orderId,
		OrderID2:   strconv.Itoa(orderId),
		Price:      ToFloat64(price),
		Amount:     ToFloat64(amount),
		DealAmount: dealAmount,
		AvgPrice:   avgPrice,
		Side:       TradeSide(side),
		Status:     ORDER_UNFINISH,
		OrderTime:  ToInt(respmap["transactTime"])}, nil
}

func (exchange *Exchange) GetAccount() (*Account, error) {
	params := url.Values{}
	exchange.buildParamsSigned(&params)
	path := exchange.apiV3 + ACCOUNT_URI + params.Encode()
	respmap, err := HttpGet2(exchange.httpClient, path, map[string]string{"X-MBX-APIKEY": exchange.accessKey})
	if err != nil {
		return nil, err
	}
	if _, isok := respmap["code"]; isok == true {
		return nil, errors.New(respmap["msg"].(string))
	}
	acc := Account{}
	acc.Exchange = exchange.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)

	balances := respmap["balances"].([]interface{})
	for _, v := range balances {
		vv := v.(map[string]interface{})
		currency := NewCurrency(vv["asset"].(string), "").AdaptBccToBch()
		acc.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       ToFloat64(vv["free"]),
			ForzenAmount: ToFloat64(vv["locked"]),
		}
	}

	return &acc, nil
}

func (exchange *Exchange) LimitBuy(amount, price string, currencyPair CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeOrder(amount, price, currencyPair, "LIMIT", "BUY")
}

func (exchange *Exchange) LimitSell(amount, price string, currencyPair CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeOrder(amount, price, currencyPair, "LIMIT", "SELL")
}

func (exchange *Exchange) MarketBuy(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return exchange.placeOrder(amount, price, currencyPair, "MARKET", "BUY")
}

func (exchange *Exchange) MarketSell(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return exchange.placeOrder(amount, price, currencyPair, "MARKET", "SELL")
}

func (exchange *Exchange) CancelOrder(orderId string, currencyPair CurrencyPair) (bool, error) {
	path := exchange.apiV3 + ORDER_URI
	params := url.Values{}
	params.Set("symbol", currencyPair.ToSymbol(""))
	params.Set("orderId", orderId)

	exchange.buildParamsSigned(&params)

	resp, err := HttpDeleteForm(exchange.httpClient, path, params, map[string]string{"X-MBX-APIKEY": exchange.accessKey})

	if err != nil {
		return false, exchange.adaptError(err)
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return false, err
	}

	orderIdCanceled := ToInt(respmap["orderId"])
	if orderIdCanceled <= 0 {
		return false, errors.New(string(resp))
	}

	return true, nil
}

func (exchange *Exchange) GetOneOrder(orderId string, currencyPair CurrencyPair) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", currencyPair.ToSymbol(""))
	if orderId != "" {
		params.Set("orderId", orderId)
	}
	params.Set("orderId", orderId)

	exchange.buildParamsSigned(&params)
	path := exchange.apiV3 + ORDER_URI + "?" + params.Encode()

	respmap, err := HttpGet2(exchange.httpClient, path, map[string]string{"X-MBX-APIKEY": exchange.accessKey})
	if err != nil {
		return nil, err
	}

	order := exchange.adaptOrder(currencyPair, respmap)

	return &order, nil
}

func (exchange *Exchange) GetUnfinishOrders(currencyPair CurrencyPair) ([]Order, error) {
	params := url.Values{}
	params.Set("symbol", currencyPair.ToSymbol(""))

	exchange.buildParamsSigned(&params)
	path := exchange.apiV3 + UNFINISHED_ORDERS_INFO + params.Encode()

	respmap, err := HttpGet3(exchange.httpClient, path, map[string]string{"X-MBX-APIKEY": exchange.accessKey})
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0)
	for _, v := range respmap {
		ord := v.(map[string]interface{})
		orders = append(orders, exchange.adaptOrder(currencyPair, ord))
	}

	return orders, nil
}

func (exchange *Exchange) GetKlineRecords(currency CurrencyPair, period KlinePeriod, size int, optional ...OptionalParameter) ([]Kline, error) {
	params := url.Values{}
	params.Set("symbol", currency.ToSymbol(""))
	params.Set("interval", _INERNAL_KLINE_PERIOD_CONVERTER[period])
	params.Set("limit", fmt.Sprintf("%d", size))
	MergeOptionalParameter(&params, optional...)

	klineUrl := exchange.apiV3 + KLINE_URI + "?" + params.Encode()
	klines, err := HttpGet3(exchange.httpClient, klineUrl, nil)
	if err != nil {
		return nil, err
	}
	var klineRecords []Kline

	for _, _record := range klines {
		r := Kline{Pair: currency}
		record := _record.([]interface{})
		r.Timestamp = int64(record[0].(float64)) / 1000 //to unix timestramp
		r.Open = ToFloat64(record[1])
		r.High = ToFloat64(record[2])
		r.Low = ToFloat64(record[3])
		r.Close = ToFloat64(record[4])
		r.Vol = ToFloat64(record[5])

		klineRecords = append(klineRecords, r)
	}

	return klineRecords, nil

}

//非个人，整个交易所的交易记录
//注意：since is fromId
func (exchange *Exchange) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	param := url.Values{}
	param.Set("symbol", currencyPair.ToSymbol(""))
	param.Set("limit", "500")
	if since > 0 {
		param.Set("fromId", strconv.Itoa(int(since)))
	}
	apiUrl := exchange.apiV3 + "historicalTrades?" + param.Encode()
	resp, err := HttpGet3(exchange.httpClient, apiUrl, map[string]string{
		"X-MBX-APIKEY": exchange.accessKey})
	if err != nil {
		return nil, err
	}

	var trades []Trade
	for _, v := range resp {
		m := v.(map[string]interface{})
		ty := SELL
		if m["isBuyerMaker"].(bool) {
			ty = BUY
		}
		trades = append(trades, Trade{
			Tid:    ToInt64(m["id"]),
			Type:   ty,
			Amount: ToFloat64(m["qty"]),
			Price:  ToFloat64(m["price"]),
			Date:   ToInt64(m["time"]),
			Pair:   currencyPair,
		})
	}

	return trades, nil
}

func (exchange *Exchange) GetOrderHistorys(currency CurrencyPair, optional ...OptionalParameter) ([]Order, error) {
	params := url.Values{}
	params.Set("symbol", currency.AdaptUsdToUsdt().ToSymbol(""))
	MergeOptionalParameter(&params, optional...)
	exchange.buildParamsSigned(&params)

	path := exchange.apiV3 + "allOrders?" + params.Encode()

	respmap, err := HttpGet3(exchange.httpClient, path, map[string]string{"X-MBX-APIKEY": exchange.accessKey})
	if err != nil {
		return nil, err
	}

	orders := make([]Order, 0)
	for _, v := range respmap {
		orderMap := v.(map[string]interface{})
		orders = append(orders, exchange.adaptOrder(currency, orderMap))
	}

	return orders, nil

}

func (exchange *Exchange) toCurrencyPair(symbol string) CurrencyPair {
	if exchange.ExchangeInfo == nil {
		var err error
		exchange.ExchangeInfo, err = exchange.GetExchangeInfo()
		if err != nil {
			return CurrencyPair{}
		}
	}
	for _, v := range exchange.ExchangeInfo.Symbols {
		if v.Symbol == symbol {
			return NewCurrencyPair2(v.BaseAsset + "_" + v.QuoteAsset)
		}
	}
	return CurrencyPair{}
}

func (exchange *Exchange) GetExchangeInfo() (*ExchangeInfo, error) {
	resp, err := HttpGet5(exchange.httpClient, exchange.apiV3+"exchangeInfo", nil)
	if err != nil {
		return nil, err
	}
	info := &ExchangeInfo{}
	err = json.Unmarshal(resp, info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (exchange *Exchange) GetTradeSymbol(currencyPair CurrencyPair) (*TradeSymbol, error) {
	if exchange.ExchangeInfo == nil {
		var err error
		exchange.ExchangeInfo, err = exchange.GetExchangeInfo()
		if err != nil {
			return nil, err
		}
	}
	for k, v := range exchange.ExchangeInfo.Symbols {
		if v.Symbol == currencyPair.ToSymbol("") {
			return &exchange.ExchangeInfo.Symbols[k], nil
		}
	}
	return nil, errors.New("symbol not found")
}

func (exchange *Exchange) adaptError(err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "Order does not exist") ||
		strings.Contains(errStr, "Unknown order sent") {
		return EX_ERR_NOT_FIND_ORDER.OriginErr(errStr)
	}

	if strings.Contains(errStr, "Too much request") {
		return EX_ERR_API_LIMIT.OriginErr(errStr)
	}

	if strings.Contains(errStr, "insufficient") {
		return EX_ERR_INSUFFICIENT_BALANCE.OriginErr(errStr)
	}

	return err
}

func (exchange *Exchange) adaptOrder(currencyPair CurrencyPair, orderMap map[string]interface{}) Order {
	side := orderMap["side"].(string)

	orderSide := SELL
	if side == "BUY" {
		orderSide = BUY
	}

	return Order{
		OrderID:      ToInt(orderMap["orderId"]),
		OrderID2:     fmt.Sprintf("%.0f", orderMap["orderId"]),
		Cid:          orderMap["clientOrderId"].(string),
		Currency:     currencyPair,
		Price:        ToFloat64(orderMap["price"]),
		Amount:       ToFloat64(orderMap["origQty"]),
		DealAmount:   ToFloat64(orderMap["executedQty"]),
		AvgPrice:     FloatToFixed(ToFloat64(orderMap["cummulativeQuoteQty"])/ToFloat64(orderMap["executedQty"]), 8),
		Side:         TradeSide(orderSide),
		Status:       adaptOrderStatus(orderMap["status"].(string)),
		OrderTime:    ToInt(orderMap["time"]),
		FinishedTime: ToInt64(orderMap["updateTime"]),
	}
}

func (exchange *Exchange) GetAssets(currency CurrencyPair) (*Assets, error) {
	panic("")
}

func (exchange *Exchange) GetTradeHistory(currency CurrencyPair, optional ...OptionalParameter) ([]Trade, error) {
	panic("")
}
