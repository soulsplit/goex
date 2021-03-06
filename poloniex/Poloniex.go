package poloniex

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	. "github.com/soulsplit/goex"
)

const EXCHANGE_NAME = "poloniex.com"

const (
	BASE_URL       = "https://poloniex.com/"
	TRADE_API      = BASE_URL + "tradingApi"
	PUBLIC_URL     = BASE_URL + "public"
	TICKER_API     = "?command=returnTicker"
	ORDER_BOOK_API = "?command=returnOrderBook&currencyPair=%s&depth=%d"
)

type Exchange struct {
	accessKey,
	secretKey string
	client *http.Client
}

func New(client *http.Client, accessKey, secretKey string) *Exchange {
	return &Exchange{accessKey, secretKey, client}
}

func (exchange *Exchange) GetExchangeName() string {
	return POLONIEX
}

func (exchange *Exchange) GetTicker(currency CurrencyPair) (*Ticker, error) {
	//log.Println(poloniex.adaptCurrencyPair(currency).ToSymbol2("_"))
	respmap, err := HttpGet(exchange.client, PUBLIC_URL+TICKER_API)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	pair := currency.AdaptUsdToUsdt().Reverse().ToSymbol("_")
	//println(pair)
	tickermap, ok := respmap[pair].(map[string]interface{})
	if !ok {
		return new(Ticker), errors.New("not found")
	}

	ticker := new(Ticker)
	ticker.Pair = currency
	ticker.High, _ = strconv.ParseFloat(tickermap["high24hr"].(string), 64)
	ticker.Low, _ = strconv.ParseFloat(tickermap["low24hr"].(string), 64)
	ticker.Last, _ = strconv.ParseFloat(tickermap["last"].(string), 64)
	ticker.Buy, _ = strconv.ParseFloat(tickermap["highestBid"].(string), 64)
	ticker.Sell, _ = strconv.ParseFloat(tickermap["lowestAsk"].(string), 64)
	ticker.Vol, _ = strconv.ParseFloat(tickermap["quoteVolume"].(string), 64)

	//log.Println(tickermap)

	return ticker, nil
}
func (exchange *Exchange) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	respmap, err := HttpGet(exchange.client, PUBLIC_URL+
		fmt.Sprintf(ORDER_BOOK_API, currency.AdaptUsdToUsdt().Reverse().ToSymbol("_"), size))

	if err != nil {
		log.Println(err)
		return nil, err
	}

	if respmap["asks"] == nil {
		log.Println(respmap)
		return nil, errors.New(fmt.Sprintf("%+v", respmap))
	}

	_, isOK := respmap["asks"].([]interface{})
	if !isOK {
		log.Println(respmap)
		return nil, errors.New(fmt.Sprintf("%+v", respmap))
	}

	var depth Depth
	depth.Pair = currency
	for _, v := range respmap["asks"].([]interface{}) {
		var dr DepthRecord
		for i, vv := range v.([]interface{}) {
			switch i {
			case 0:
				dr.Price, _ = strconv.ParseFloat(vv.(string), 64)
			case 1:
				dr.Amount = vv.(float64)
			}
		}
		depth.AskList = append(depth.AskList, dr)
	}

	for _, v := range respmap["bids"].([]interface{}) {
		var dr DepthRecord
		for i, vv := range v.([]interface{}) {
			switch i {
			case 0:
				dr.Price, _ = strconv.ParseFloat(vv.(string), 64)
			case 1:
				dr.Amount = vv.(float64)
			}
		}
		depth.BidList = append(depth.BidList, dr)
	}

	return &depth, nil
}
func (exchange *Exchange) GetKlineRecords(currency CurrencyPair, period KlinePeriod, size int, optional ...OptionalParameter) ([]Kline, error) {
	return nil, nil
}

func (exchange *Exchange) placeLimitOrder(command, amount, price string, currency CurrencyPair) (*Order, error) {
	postData := url.Values{}
	postData.Set("command", command)
	postData.Set("currencyPair", currency.AdaptUsdToUsdt().Reverse().ToSymbol("_"))
	postData.Set("rate", price)
	postData.Set("amount", amount)

	sign, _ := exchange.buildPostForm(&postData)

	headers := map[string]string{
		"Key":  exchange.accessKey,
		"Sign": sign}

	resp, err := HttpPostForm2(exchange.client, TRADE_API, postData, headers)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil || respmap["error"] != nil {
		log.Println(err, string(resp))
		return nil, err
	}

	orderNumber := respmap["orderNumber"].(string)
	order := new(Order)
	order.OrderTime = int(time.Now().Unix() * 1000)
	order.OrderID, _ = strconv.Atoi(orderNumber)
	order.OrderID2 = orderNumber
	order.Amount, _ = strconv.ParseFloat(amount, 64)
	order.Price, _ = strconv.ParseFloat(price, 64)
	order.Status = ORDER_UNFINISH
	order.Currency = currency

	switch command {
	case "sell":
		order.Side = SELL
	case "buy":
		order.Side = BUY
	}

	//log.Println(string(resp))
	return order, nil
}

func (exchange *Exchange) LimitBuy(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeLimitOrder("buy", amount, price, currency)
}

func (exchange *Exchange) LimitSell(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeLimitOrder("sell", amount, price, currency)
}

func (exchange *Exchange) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	postData := url.Values{}
	postData.Set("command", "cancelOrder")
	postData.Set("orderNumber", orderId)

	sign, err := exchange.buildPostForm(&postData)
	if err != nil {
		log.Println(err)
		return false, err
	}

	headers := map[string]string{
		"Key":  exchange.accessKey,
		"Sign": sign}
	resp, err := HttpPostForm2(exchange.client, TRADE_API, postData, headers)
	if err != nil {
		log.Println(err)
		return false, err
	}

	//log.Println(string(resp));

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil || respmap["error"] != nil {
		//log.Println(err, string(resp))
		return false, errors.New(string(resp))
	}

	success := int(respmap["success"].(float64))
	if success != 1 {
		log.Println(respmap)
		return false, nil
	}

	return true, nil
}

func (exchange *Exchange) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	postData := url.Values{}
	postData.Set("command", "returnOrderTrades")
	postData.Set("orderNumber", orderId)

	sign, _ := exchange.buildPostForm(&postData)

	headers := map[string]string{
		"Key":  exchange.accessKey,
		"Sign": sign}

	resp, err := HttpPostForm2(exchange.client, TRADE_API, postData, headers)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	//println(string(resp))
	if strings.Contains(string(resp), "error") {
		ords, err1 := exchange.GetUnfinishOrders(currency)
		if err1 != nil {
			log.Println(err1)
			if strings.Contains(err1.Error(), "Order not found") {
				return nil, EX_ERR_NOT_FIND_ORDER
			}
		} else {
			_ordId, _ := strconv.Atoi(orderId)

			for _, ord := range ords {
				if ord.OrderID == _ordId {
					return &ord, nil
				}
			}
		}
		//log.Println(string(resp))
		return nil, errors.New(string(resp))
	}

	respmap := make([]interface{}, 0)
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		log.Println(err, string(resp))
		return nil, err
	}

	order := new(Order)
	order.OrderID, _ = strconv.Atoi(orderId)
	order.Currency = currency

	total := 0.0

	for _, v := range respmap {
		vv := v.(map[string]interface{})
		_amount, _ := strconv.ParseFloat(vv["amount"].(string), 64)
		_rate, _ := strconv.ParseFloat(vv["rate"].(string), 64)
		_fee, _ := strconv.ParseFloat(vv["fee"].(string), 64)

		order.DealAmount += _amount
		total += (_amount * _rate)
		order.Fee = _fee

		if strings.Compare("sell", vv["type"].(string)) == 0 {
			order.Side = TradeSide(SELL)
		} else {
			order.Side = TradeSide(BUY)
		}
	}

	order.AvgPrice = total / order.DealAmount

	return order, nil
}

func (exchange *Exchange) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	postData := url.Values{}
	postData.Set("command", "returnOpenOrders")
	postData.Set("currencyPair", currency.AdaptUsdToUsdt().Reverse().ToSymbol("_"))

	sign, err := exchange.buildPostForm(&postData)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	headers := map[string]string{
		"Key":  exchange.accessKey,
		"Sign": sign}
	resp, err := HttpPostForm2(exchange.client, TRADE_API, postData, headers)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	orderAr := make([]interface{}, 1)
	err = json.Unmarshal(resp, &orderAr)
	if err != nil {
		log.Println(err, string(resp))
		return nil, err
	}

	orders := make([]Order, 0)
	for _, v := range orderAr {
		vv := v.(map[string]interface{})
		order := Order{}
		order.Currency = currency
		order.OrderID, _ = strconv.Atoi(vv["orderNumber"].(string))
		order.OrderID2 = vv["orderNumber"].(string)
		order.Amount, _ = strconv.ParseFloat(vv["amount"].(string), 64)
		order.Price, _ = strconv.ParseFloat(vv["rate"].(string), 64)
		order.Status = ORDER_UNFINISH

		side := vv["type"].(string)
		switch side {
		case "buy":
			order.Side = TradeSide(BUY)
		case "sell":
			order.Side = TradeSide(SELL)
		}

		orders = append(orders, order)
	}

	//log.Println(orders)
	return orders, nil
}
func (exchange *Exchange) GetOrderHistorys(currency CurrencyPair, opt ...OptionalParameter) ([]Order, error) {
	return nil, nil
}

func (exchange *Exchange) GetAccount() (*Account, error) {
	postData := url.Values{}
	postData.Add("command", "returnCompleteBalances")
	sign, err := exchange.buildPostForm(&postData)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Key":  exchange.accessKey,
		"Sign": sign}
	resp, err := HttpPostForm2(exchange.client, TRADE_API, postData, headers)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)

	if err != nil || respmap["error"] != nil {
		log.Println(err)
		return nil, err
	}

	acc := new(Account)
	acc.Exchange = EXCHANGE_NAME
	acc.SubAccounts = make(map[Currency]SubAccount)

	for k, v := range respmap {
		var currency Currency = NewCurrency(k, "")
		vv := v.(map[string]interface{})
		subAcc := SubAccount{}
		subAcc.Currency = currency
		subAcc.Amount, _ = strconv.ParseFloat(vv["available"].(string), 64)
		subAcc.ForzenAmount, _ = strconv.ParseFloat(vv["onOrders"].(string), 64)
		acc.SubAccounts[subAcc.Currency] = subAcc
		if currency.Symbol == "USDT" {
			acc.SubAccounts[USD] = subAcc
		}
	}

	return acc, nil
}

func (exchange *Exchange) Withdraw(amount string, currency Currency, fees, receiveAddr, safePwd string) (string, error) {
	if currency == BCC {
		currency = BCH
	}
	params := url.Values{}
	params.Add("command", "withdraw")
	params.Add("address", receiveAddr)
	params.Add("amount", amount)
	params.Add("currency", strings.ToUpper(currency.String()))

	sign, err := exchange.buildPostForm(&params)
	if err != nil {
		return "", err
	}

	headers := map[string]string{
		"Key":  exchange.accessKey,
		"Sign": sign}

	resp, err := HttpPostForm2(exchange.client, TRADE_API, params, headers)

	if err != nil {
		log.Println(err)
		return "", err
	}
	println(string(resp))

	respMap := make(map[string]interface{})

	err = json.Unmarshal(resp, &respMap)
	if err != nil {
		log.Println(err)
		return "", err
	}

	if respMap["error"] == nil {
		return string(resp), nil
	}

	return "", errors.New(string(resp))
}

type PoloniexDepositsWithdrawals struct {
	Deposits []struct {
		Currency      string    `json:"currency"`
		Address       string    `json:"address"`
		Amount        float64   `json:"amount,string"`
		Confirmations int       `json:"confirmations"`
		TransactionID string    `json:"txid"`
		Timestamp     time.Time `json:"timestamp"`
		Status        string    `json:"status"`
	} `json:"deposits"`
	Withdrawals []struct {
		WithdrawalNumber int64     `json:"withdrawalNumber"`
		Currency         string    `json:"currency"`
		Address          string    `json:"address"`
		Amount           float64   `json:"amount,string"`
		Confirmations    int       `json:"confirmations"`
		TransactionID    string    `json:"txid"`
		Timestamp        time.Time `json:"timestamp"`
		Status           string    `json:"status"`
		IPAddress        string    `json:"ipAddress"`
	} `json:"withdrawals"`
}

func (exchange *Exchange) GetDepositsWithdrawals(start, end string) (*PoloniexDepositsWithdrawals, error) {
	params := url.Values{}
	params.Set("command", "returnDepositsWithdrawals")
	println(start)
	if start != "" {
		params.Set("start", start)
	} else {
		params.Set("start", "0")
	}

	if end != "" {
		params.Set("end", end)
	} else {
		params.Set("end", strconv.FormatInt(time.Now().Unix(), 10))
	}

	sign, err := exchange.buildPostForm(&params)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Key":  exchange.accessKey,
		"Sign": sign}

	resp, err := HttpPostForm2(exchange.client, TRADE_API, params, headers)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	println(string(resp))

	records := new(PoloniexDepositsWithdrawals)
	err = json.Unmarshal(resp, records)

	return records, err
}

func (exchange *Exchange) buildPostForm(postForm *url.Values) (string, error) {
	postForm.Add("nonce", fmt.Sprintf("%d", time.Now().UnixNano()))
	payload := postForm.Encode()
	//println(payload)
	sign, err := GetParamHmacSHA512Sign(exchange.secretKey, payload)
	if err != nil {
		return "", err
	}
	//log.Println(sign)
	return sign, nil
}

func (exchange *Exchange) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("unimplements")
}

func (exchange *Exchange) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	panic("unsupport the market order")
}

func (exchange *Exchange) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	panic("unsupport the market order")
}

func (exchange *Exchange) GetAssets(currency CurrencyPair) (*Assets, error) {
	panic("")
}

func (exchange *Exchange) GetTradeHistory(currency CurrencyPair, optional ...OptionalParameter) ([]Trade, error) {
	panic("")
}
