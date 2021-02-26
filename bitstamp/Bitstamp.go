package bitstamp

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

var (
	BASE_URL = "https://www.bitstamp.net/api/"
)

type Exchange struct {
	client *http.Client
	clientId,
	accessKey,
	secretkey string
}

func NewBitstamp(client *http.Client, accessKey, secertkey, clientId string) *Exchange {
	return &Exchange{client: client, accessKey: accessKey, secretkey: secertkey, clientId: clientId}
}

func (exchange *Exchange) buildPostForm(params *url.Values) {
	nonce := time.Now().UnixNano()
	//println(nonce)
	payload := fmt.Sprintf("%d%s%s", nonce, exchange.clientId, exchange.accessKey)
	sign, _ := GetParamHmacSHA256Sign(exchange.secretkey, payload)
	params.Set("signature", strings.ToUpper(sign))
	params.Set("nonce", fmt.Sprintf("%d", nonce))
	params.Set("key", exchange.accessKey)
}

func (exchange *Exchange) GetAccount() (*Account, error) {
	urlStr := fmt.Sprintf("%s%s", BASE_URL, "v2/balance/")
	params := url.Values{}
	exchange.buildPostForm(&params)
	resp, err := HttpPostForm(exchange.client, urlStr, params)
	if err != nil {
		return nil, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	acc := Account{}
	acc.Exchange = exchange.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)
	acc.SubAccounts[BTC] = SubAccount{
		Currency:     BTC,
		Amount:       ToFloat64(respmap["btc_available"]),
		ForzenAmount: ToFloat64(respmap["btc_reserved"]),
		LoanAmount:   0,
	}
	acc.SubAccounts[LTC] = SubAccount{
		Currency:     LTC,
		Amount:       ToFloat64(respmap["ltc_available"]),
		ForzenAmount: ToFloat64(respmap["ltc_reserved"]),
		LoanAmount:   0,
	}
	acc.SubAccounts[ETH] = SubAccount{
		Currency:     ETH,
		Amount:       ToFloat64(respmap["eth_available"]),
		ForzenAmount: ToFloat64(respmap["eth_reserved"]),
		LoanAmount:   0,
	}
	acc.SubAccounts[XRP] = SubAccount{
		Currency:     XRP,
		Amount:       ToFloat64(respmap["xrp_available"]),
		ForzenAmount: ToFloat64(respmap["xrp_reserved"]),
		LoanAmount:   0,
	}
	acc.SubAccounts[USD] = SubAccount{
		Currency:     USD,
		Amount:       ToFloat64(respmap["usd_available"]),
		ForzenAmount: ToFloat64(respmap["usd_reserved"]),
		LoanAmount:   0,
	}
	acc.SubAccounts[EUR] = SubAccount{
		Currency:     EUR,
		Amount:       ToFloat64(respmap["eur_available"]),
		ForzenAmount: ToFloat64(respmap["eur_reserved"]),
		LoanAmount:   0,
	}
	acc.SubAccounts[BCH] = SubAccount{
		Currency:     BCH,
		Amount:       ToFloat64(respmap["bch_available"]),
		ForzenAmount: ToFloat64(respmap["bch_reserved"]),
		LoanAmount:   0}
	acc.SubAccounts[GBP] = SubAccount{
		Currency:     GBP,
		Amount:       ToFloat64(respmap["gbp_available"]),
		ForzenAmount: ToFloat64(respmap["gbp_reserved"]),
		LoanAmount:   0}
	acc.SubAccounts[PAX] = SubAccount{
		Currency:     PAX,
		Amount:       ToFloat64(respmap["pax_available"]),
		ForzenAmount: ToFloat64(respmap["pax_reserved"]),
		LoanAmount:   0}
	acc.SubAccounts[XLM] = SubAccount{
		Currency:     XLM,
		Amount:       ToFloat64(respmap["xlm_available"]),
		ForzenAmount: ToFloat64(respmap["xlm_reserved"]),
		LoanAmount:   0}
	return &acc, nil
}

func (exchange *Exchange) placeOrder(side string, pair CurrencyPair, amount, price, urlStr string) (*Order, error) {
	params := url.Values{}
	params.Set("amount", amount)
	if price != "" {
		params.Set("price", price)
	}
	exchange.buildPostForm(&params)

	resp, err := HttpPostForm(exchange.client, urlStr, params)
	if err != nil {
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	orderId, isok := respmap["id"].(string)
	if !isok {
		return nil, errors.New(string(resp))
	}

	orderSide := BUY
	if side == "sell" {
		orderSide = SELL
	}

	orderprice, isok := respmap["price"].(string)
	if !isok {
		return nil, errors.New(string(resp))
	}

	return &Order{
		Currency:   pair,
		OrderID:    ToInt(orderId),
		OrderID2:   orderId,
		Price:      ToFloat64(orderprice),
		Amount:     ToFloat64(amount),
		DealAmount: 0,
		AvgPrice:   0,
		Side:       TradeSide(orderSide),
		Status:     ORDER_UNFINISH,
		OrderTime:  1}, nil
}

func (exchange *Exchange) placeLimitOrder(side string, pair CurrencyPair, amount, price string) (*Order, error) {
	urlStr := fmt.Sprintf("%sv2/%s/%s/", BASE_URL, side, strings.ToLower(pair.ToSymbol("")))
	//println(urlStr)
	return exchange.placeOrder(side, pair, amount, price, urlStr)
}

func (exchange *Exchange) placeMarketOrder(side string, pair CurrencyPair, amount string) (*Order, error) {
	urlStr := fmt.Sprintf("%sv2/%s/market/%s/", BASE_URL, side, strings.ToLower(pair.ToSymbol("")))
	//println(urlStr)
	return exchange.placeOrder(side, pair, amount, "", urlStr)
}

func (exchange *Exchange) LimitBuy(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeLimitOrder("buy", currency, amount, price)
}

func (exchange *Exchange) LimitSell(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeLimitOrder("sell", currency, amount, price)
}

func (exchange *Exchange) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	return exchange.placeMarketOrder("buy", currency, amount)
}

func (exchange *Exchange) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return exchange.placeMarketOrder("sell", currency, amount)
}

func (exchange *Exchange) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	params := url.Values{}
	params.Set("id", orderId)
	exchange.buildPostForm(&params)

	urlStr := BASE_URL + "v2/cancel_order/"
	resp, err := HttpPostForm(exchange.client, urlStr, params)
	if err != nil {
		return false, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return false, err
	}

	if respmap["error"] != nil {
		return false, errors.New(string(resp))
	}

	println(string(resp))
	return true, nil
}

func (exchange *Exchange) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	params := url.Values{}
	params.Set("id", orderId)
	exchange.buildPostForm(&params)

	urlStr := BASE_URL + "order_status/"
	resp, err := HttpPostForm(exchange.client, urlStr, params)
	if err != nil {
		return nil, err
	}
	//println(string(resp))
	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}

	transactions, isok := respmap["transactions"].([]interface{})
	if !isok {
		return nil, errors.New(string(resp))
	}

	status := respmap["status"].(string)

	ord := Order{}
	ord.Currency = currency
	ord.OrderID = ToInt(orderId)
	ord.OrderID2 = orderId

	if status == "Finished" {
		ord.Status = ORDER_FINISH
	} else {
		ord.Status = ORDER_UNFINISH
	}

	if len(transactions) > 0 {
		if ord.Status != ORDER_FINISH {
			ord.Status = ORDER_PART_FINISH
		}

		var (
			dealAmount  float64
			tradeAmount float64
			fee         float64
		)

		currencyStr := strings.ToLower(currency.CurrencyA.Symbol)
		for _, v := range transactions {
			transaction := v.(map[string]interface{})
			price := ToFloat64(transaction["price"])
			amount := ToFloat64(transaction[currencyStr])
			dealAmount += amount
			tradeAmount += amount * price
			fee += ToFloat64(transaction["fee"])
			//tpy := ToInt(transaction["type"]) //注意:不是交易方向，type (0 - deposit; 1 - withdrawal; 2 - market trade)
			//if tpy == 2 {
			//	ord.Side = SELL
			//}
		}

		avgPrice := tradeAmount / dealAmount
		ord.DealAmount = dealAmount
		ord.AvgPrice = avgPrice
		ord.Fee = fee
	}

	//	println(string(resp))
	return &ord, nil
}

func (exchange *Exchange) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	params := url.Values{}
	exchange.buildPostForm(&params)

	urlStr := BASE_URL + "v2/open_orders/" + strings.ToLower(currency.ToSymbol("")) + "/"
	resp, err := HttpPostForm(exchange.client, urlStr, params)
	if err != nil {
		return nil, err
	}

	respmap := make([]interface{}, 1)
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return nil, err
	}
	orders := make([]Order, 0)
	for _, v := range respmap {
		ord := v.(map[string]interface{})
		side := ToInt(ord["type"])
		orderSide := SELL
		if side == 0 {
			orderSide = BUY
		}
		orderTime, _ := time.Parse("2006-01-02 15:04:05", ord["datetime"].(string))
		orders = append(orders, Order{
			OrderID:   ToInt(ord["id"]),
			OrderID2:  fmt.Sprint(ToInt(ord["id"])),
			Currency:  currency,
			Price:     ToFloat64(ord["price"]),
			Amount:    ToFloat64(ord["amount"]),
			Side:      TradeSide(orderSide),
			Status:    ORDER_UNFINISH,
			OrderTime: int(orderTime.Unix())})
	}
	//println(string(resp))

	return orders, nil
}

func (exchange *Exchange) GetOrderHistorys(currency CurrencyPair, optional ...OptionalParameter) ([]Order, error) {
	panic("not implement")
}

//

func (exchange *Exchange) GetTicker(currency CurrencyPair) (*Ticker, error) {
	urlStr := BASE_URL + "v2/ticker/" + strings.ToLower(currency.ToSymbol(""))
	respmap, err := HttpGet(exchange.client, urlStr)
	if err != nil {
		return nil, err
	}
	timestamp, _ := strconv.ParseUint(respmap["timestamp"].(string), 10, 64)
	return &Ticker{
		Pair: currency,
		Last: ToFloat64(respmap["last"]),
		High: ToFloat64(respmap["high"]),
		Low:  ToFloat64(respmap["low"]),
		Vol:  ToFloat64(respmap["volume"]),
		Sell: ToFloat64(respmap["ask"]),
		Buy:  ToFloat64(respmap["bid"]),
		Date: timestamp}, nil
}

func (exchange *Exchange) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	urlStr := BASE_URL + "v2/order_book/" + strings.ToLower(currency.ToSymbol(""))
	respmap, err := HttpGet(exchange.client, urlStr)
	if err != nil {
		return nil, err
	}

	//timestamp, _ := strconv.ParseUint(respmap["timestamp"].(string), 10, 64)
	bids, isok1 := respmap["bids"].([]interface{})
	asks, isok2 := respmap["asks"].([]interface{})
	if !isok1 || !isok2 {
		return nil, errors.New("Get Depth Error.")
	}

	i := 0
	dep := new(Depth)
	dep.Pair = currency
	for _, v := range bids {
		bid := v.([]interface{})
		dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(bid[0]), ToFloat64(bid[1])})
		i++
		if i == size {
			break
		}
	}

	i = 0
	for _, v := range asks {
		ask := v.([]interface{})
		dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(ask[0]), ToFloat64(ask[1])})
		i++
		if i == size {
			break
		}
	}

	sort.Sort(sort.Reverse(dep.AskList)) //reverse
	return dep, nil
}

func (exchange *Exchange) GetKlineRecords(currency CurrencyPair, period KlinePeriod, size int, optional ...OptionalParameter) ([]Kline, error) {
	panic("not implement")
}

////非个人，整个交易所的交易记录
func (exchange *Exchange) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("not implement")
}

func (exchange *Exchange) GetExchangeName() string {
	return BITSTAMP
}

func (exchange *Exchange) GetAssets(currency CurrencyPair) (*Assets, error) {
	panic("")
}

func (exchange *Exchange) GetTradeHistory(currency CurrencyPair, optional ...OptionalParameter) ([]Trade, error) {
	panic("")
}
