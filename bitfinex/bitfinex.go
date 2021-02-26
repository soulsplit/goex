package bitfinex

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/soulsplit/goex"
)

type Exchange struct {
	httpClient *http.Client
	accessKey,
	secretKey string
}

const (
	baseURL  string = "https://api.bitfinex.com"
	apiURLV1 string = baseURL + "/v1"
	apiURLV2 string = baseURL + "/v2"
)

func New(client *http.Client, accessKey, secretKey string) *Exchange {
	return &Exchange{client, accessKey, secretKey}
}

func (exchange *Exchange) GetExchangeName() string {
	return BITFINEX
}

func (exchange *Exchange) GetTicker(currencyPair CurrencyPair) (*Ticker, error) {
	//pubticker
	currencyPair = exchange.adaptCurrencyPair(currencyPair)

	apiUrl := fmt.Sprintf("%s/pubticker/%s", apiURLV1, strings.ToLower(currencyPair.ToSymbol("")))
	resp, err := HttpGet(exchange.httpClient, apiUrl)
	if err != nil {
		return nil, err
	}

	if resp["error"] != nil {
		return nil, fmt.Errorf("%s", resp["error"])
	}

	//fmt.Println(resp)
	ticker := new(Ticker)
	ticker.Pair = currencyPair
	ticker.Last = ToFloat64(resp["last_price"])
	ticker.Vol = ToFloat64(resp["volume"])
	ticker.High = ToFloat64(resp["high"])
	ticker.Low = ToFloat64(resp["low"])
	ticker.Sell = ToFloat64(resp["ask"])
	ticker.Buy = ToFloat64(resp["bid"])
	ticker.Date = uint64(exchange.adaptTimestamp(resp["timestamp"].(string)))
	return ticker, nil
}

func (exchange *Exchange) GetDepth(size int, currencyPair CurrencyPair) (*Depth, error) {
	apiUrl := fmt.Sprintf("%s/book/%s?limit_bids=%d&limit_asks=%d", apiURLV1, exchange.currencyPairToSymbol(currencyPair), size, size)
	resp, err := HttpGet(exchange.httpClient, apiUrl)
	if err != nil {
		return nil, err
	}
	bids := resp["bids"].([]interface{})
	asks := resp["asks"].([]interface{})

	depth := new(Depth)

	for _, bid := range bids {
		_bid := bid.(map[string]interface{})
		amount := ToFloat64(_bid["amount"])
		price := ToFloat64(_bid["price"])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.BidList = append(depth.BidList, dr)
	}

	for _, ask := range asks {
		_ask := ask.(map[string]interface{})
		amount := ToFloat64(_ask["amount"])
		price := ToFloat64(_ask["price"])
		dr := DepthRecord{Amount: amount, Price: price}
		depth.AskList = append(depth.AskList, dr)
	}

	return depth, nil
}

func (exchange *Exchange) GetKlineRecords(currencyPair CurrencyPair, period KlinePeriod, size int, optional ...OptionalParameter) ([]Kline, error) {
	symbol := convertPairToBitfinexSymbol("t", currencyPair)
	if size == 0 {
		size = 100
	}

	periodStr, ok := klinePeriods[period]
	if !ok {
		return nil, fmt.Errorf("invalid period")
	}
	apiURL := fmt.Sprintf("%s/candles/trade:%s:%s/hist?limit=%d", apiURLV2, periodStr, symbol, size)

	respRaw, err := NewHttpRequest(exchange.httpClient, "GET", apiURL, "", nil)
	if err != nil {
		return nil, err
	}

	var candles []Kline

	var resp [][]interface{}
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal kline response: %v", err)
	}

	for _, r := range resp {
		k := klineFromRaw(currencyPair, r)
		candles = append(candles, *k)
	}

	return candles, nil
}

//非个人，整个交易所的交易记录

func (exchange *Exchange) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("not implement")
}

func (exchange *Exchange) GetWalletBalances() (map[string]*Account, error) {
	var respmap []interface{}
	err := exchange.doAuthenticatedRequest("GET", "balances", map[string]interface{}{}, &respmap)
	if err != nil {
		return nil, err
	}

	walletmap := make(map[string]*Account, 1)

	for _, v := range respmap {
		subacc := v.(map[string]interface{})
		typeStr := subacc["type"].(string)

		currency := NewCurrency(subacc["currency"].(string), "")

		if currency == UNKNOWN {
			continue
		}

		//typeS := subacc["type"].(string)
		amount := ToFloat64(subacc["amount"])
		available := ToFloat64(subacc["available"])

		account := walletmap[typeStr]
		if account == nil {
			account = new(Account)
			account.SubAccounts = make(map[Currency]SubAccount, 6)
		}

		account.NetAsset = amount
		account.Asset = amount
		account.SubAccounts[currency] = SubAccount{
			Currency:     currency,
			Amount:       available,
			ForzenAmount: amount - available,
			LoanAmount:   0}

		walletmap[typeStr] = account
	}

	return walletmap, nil
}

/*defalut only return exchange wallet balance*/
func (bfx *Exchange) GetAccount() (*Account, error) {
	wallets, err := bfx.GetWalletBalances()
	if err != nil {
		return nil, err
	}
	return wallets["exchange"], nil
}

func (exchange *Exchange) placeOrder(orderType, side, amount, price string, pair CurrencyPair) (*Order, error) {
	path := "order/new"
	params := map[string]interface{}{
		"symbol":   exchange.currencyPairToSymbol(pair),
		"amount":   amount,
		"price":    price,
		"side":     side,
		"type":     orderType,
		"exchange": "bitfinex"}

	var respmap map[string]interface{}
	err := exchange.doAuthenticatedRequest("POST", path, params, &respmap)
	if err != nil {
		return nil, err
	}

	order := new(Order)
	order.Currency = pair
	order.OrderID = ToInt(respmap["id"])
	order.OrderID2 = fmt.Sprint(ToInt(respmap["id"]))
	order.Amount = ToFloat64(amount)
	order.Price = ToFloat64(price)
	order.AvgPrice = ToFloat64(respmap["avg_execution_price"])
	order.DealAmount = ToFloat64(respmap["executed_amount"])
	order.Status = ORDER_UNFINISH

	switch side {
	case "buy":
		order.Side = BUY
		if strings.Contains(orderType, "market") {
			order.Side = BUY_MARKET
		}
	case "sell":
		order.Side = SELL
		if strings.Contains(orderType, "market") {
			order.Side = SELL_MARKET
		}
	}
	return order, nil
}

func (exchange *Exchange) LimitBuy(amount, price string, currencyPair CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeOrder("exchange limit", "buy", amount, price, currencyPair)
}

func (exchange *Exchange) LimitSell(amount, price string, currencyPair CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeOrder("exchange limit", "sell", amount, price, currencyPair)
}

func (exchange *Exchange) MarketBuy(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return exchange.placeOrder("exchange market", "buy", amount, price, currencyPair)
}

func (exchange *Exchange) MarketSell(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return exchange.placeOrder("exchange market", "sell", amount, price, currencyPair)
}

func (exchange *Exchange) StopBuy(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return exchange.placeOrder("exchange stop", "buy", amount, price, currencyPair)
}

func (exchange *Exchange) StopSell(amount, price string, currencyPair CurrencyPair) (*Order, error) {
	return exchange.placeOrder("exchange stop", "sell", amount, price, currencyPair)
}

func (exchange *Exchange) CancelOrder(orderId string, currencyPair CurrencyPair) (bool, error) {
	var respmap map[string]interface{}
	path := "order/cancel"
	err := exchange.doAuthenticatedRequest("POST", path, map[string]interface{}{"order_id": ToInt(orderId)}, &respmap)
	if err != nil {
		return false, err
	}
	return respmap["is_cancelled"].(bool), nil
}

func (exchange *Exchange) toOrder(respmap map[string]interface{}) *Order {
	order := new(Order)
	order.Currency = symbolToCurrencyPair(respmap["symbol"].(string))
	order.OrderID = ToInt(respmap["id"])
	order.OrderID2 = fmt.Sprint(ToInt(respmap["id"]))
	order.Amount = ToFloat64(respmap["original_amount"])
	order.Price = ToFloat64(respmap["price"])
	order.DealAmount = ToFloat64(respmap["executed_amount"])
	order.AvgPrice = ToFloat64(respmap["avg_execution_price"])
	order.OrderTime = exchange.adaptTimestamp(respmap["timestamp"].(string))

	if order.DealAmount == order.Amount {
		order.Status = ORDER_FINISH
	} else if order.DealAmount > 0 {
		order.Status = ORDER_PART_FINISH
	}

	side := respmap["side"].(string)
	if side == "sell" {
		order.Side = SELL
	} else if side == "buy" {
		order.Side = BUY
	}

	if respmap["is_cancelled"].(bool) {
		order.Status = ORDER_CANCEL
	}
	return order
}

func (exchange *Exchange) GetOneOrder(orderId string, currencyPair CurrencyPair) (*Order, error) {
	var respmap map[string]interface{}
	path := "order/status"
	err := exchange.doAuthenticatedRequest("POST", path, map[string]interface{}{"order_id": ToInt(orderId)}, &respmap)
	if err != nil {
		return nil, err
	}
	return exchange.toOrder(respmap), nil
}

func (exchange *Exchange) GetUnfinishOrders(currencyPair CurrencyPair) ([]Order, error) {
	var ordersmap []interface{}
	err := exchange.doAuthenticatedRequest("POST", "orders", map[string]interface{}{}, &ordersmap)
	if err != nil {
		return nil, err
	}

	var orders []Order
	for _, v := range ordersmap {
		ordermap := v.(map[string]interface{})
		orders = append(orders, *exchange.toOrder(ordermap))
	}
	return orders, nil
}

func (exchange *Exchange) GetOrderHistorys(currencyPair CurrencyPair, optional ...OptionalParameter) ([]Order, error) {
	panic("not implement")
}

func (exchange *Exchange) doAuthenticatedRequest(method, path string, payload map[string]interface{}, ret interface{}) error {
	nonce := time.Now().UnixNano()
	payload["request"] = "/v1/" + path
	payload["nonce"] = fmt.Sprintf("%d.2", nonce)

	p, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(p)
	sign, _ := GetParamHmacSha384Sign(exchange.secretKey, encoded)

	resp, err := NewHttpRequest(exchange.httpClient, method, apiURLV1+"/"+path, "", map[string]string{
		"Content-Type":    "application/json",
		"Accept":          "application/json",
		"X-BFX-APIKEY":    exchange.accessKey,
		"X-BFX-PAYLOAD":   encoded,
		"X-BFX-SIGNATURE": sign})

	if err != nil {
		return err
	}
	//print(string(resp))
	err = json.Unmarshal(resp, ret)
	return err
}

func (exchange *Exchange) currencyPairToSymbol(currencyPair CurrencyPair) string {
	return strings.ToUpper(currencyPair.ToSymbol(""))
}

func (exchange *Exchange) adaptTimestamp(timestamp string) int {
	times := strings.Split(timestamp, ".")
	intTime, _ := strconv.Atoi(times[0])
	return intTime
}

func (exchange *Exchange) adaptCurrencyPair(pair CurrencyPair) CurrencyPair {
	var currencyA Currency
	var currencyB Currency

	DASH := NewCurrency("DASH", "")
	DSH := NewCurrency("DSH", "")
	QTM := NewCurrency("QTM", "")
	IOTA := NewCurrency("IOTA", "")
	IOT := NewCurrency("IOT", "")

	if pair.CurrencyA == DASH {
		currencyA = DSH
	} else if pair.CurrencyA == QTUM {
		currencyA = QTM
	} else if pair.CurrencyA == IOTA {
		currencyA = IOT
	} else {
		currencyA = pair.CurrencyA
	}

	if pair.CurrencyB == USDT {
		currencyB = USD
	} else {
		currencyB = pair.CurrencyB
	}

	return NewCurrencyPair(currencyA, currencyB)
}

func symbolToCurrencyPair(symbol string) CurrencyPair {
	currencyA := strings.ToUpper(symbol[0:3])
	currencyB := strings.ToUpper(symbol[3:])
	return NewCurrencyPair(NewCurrency(currencyA, ""), NewCurrency(currencyB, ""))
}

var klinePeriods = map[KlinePeriod]string{
	KLINE_PERIOD_1MIN:   "1m",
	KLINE_PERIOD_5MIN:   "5m",
	KLINE_PERIOD_15MIN:  "15m",
	KLINE_PERIOD_30MIN:  "30m",
	KLINE_PERIOD_60MIN:  "1h",
	KLINE_PERIOD_3H:     "3h",
	KLINE_PERIOD_6H:     "6h",
	KLINE_PERIOD_12H:    "12h",
	KLINE_PERIOD_1DAY:   "1D",
	KLINE_PERIOD_1WEEK:  "7D",
	KLINE_PERIOD_1MONTH: "1M",
}

func klineFromRaw(pair CurrencyPair, raw []interface{}) *Kline {
	return &Kline{
		Pair:      pair,
		Timestamp: ToInt64(raw[0]),
		Open:      ToFloat64(raw[1]),
		Close:     ToFloat64(raw[2]),
		High:      ToFloat64(raw[3]),
		Low:       ToFloat64(raw[4]),
		Vol:       ToFloat64(raw[5]),
	}
}

func convertPairToBitfinexSymbol(prefix string, pair CurrencyPair) string {
	symbol := pair.ToSymbol("")
	return prefix + symbol
}

func convertKeyToPair(key string) CurrencyPair {
	split := strings.Split(key, ":")
	return symbolToCurrencyPair(split[2][1:])
}

func (exchange *Exchange) GetAssets(currency CurrencyPair) (*Assets, error) {
	panic("")
}

func (exchange *Exchange) GetTradeHistory(currency CurrencyPair, optional ...OptionalParameter) ([]Trade, error) {
	panic("")
}
