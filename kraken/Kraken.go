package kraken

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	. "github.com/soulsplit/goex"
)

type BaseResponse struct {
	Error  []string    `json:"error"`
	Result interface{} `json:"result"`
}

type NewOrderResponse struct {
	Description interface{} `json:"descr"`
	TxIds       []string    `json:"txid"`
}

type Exchange struct {
	httpClient *http.Client
	accessKey,
	secretKey string
}

var (
	BASE_URL   = "https://api.kraken.com"
	API_V0     = "/0/"
	API_DOMAIN = BASE_URL + API_V0
	PUBLIC     = "public/"
	PRIVATE    = "private/"
)

func New(client *http.Client, accesskey, secretkey string) *Exchange {
	return &Exchange{client, accesskey, secretkey}
}

func (exchange *Exchange) placeOrder(orderType, side, amount, price string, pair CurrencyPair) (*Order, error) {
	apiuri := PRIVATE + "AddOrder"

	params := url.Values{}
	params.Set("pair", exchange.convertPair(pair).ToSymbol(""))
	params.Set("type", side)
	params.Set("ordertype", orderType)
	params.Set("price", price)
	params.Set("volume", amount)

	var resp NewOrderResponse
	err := exchange.doAuthenticatedRequest("POST", apiuri, params, &resp)
	//log.Println
	if err != nil {
		return nil, err
	}

	var tradeSide TradeSide = SELL
	if "buy" == side {
		tradeSide = BUY
	}

	return &Order{
		Currency: pair,
		OrderID2: resp.TxIds[0],
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     tradeSide,
		Status:   ORDER_UNFINISH}, nil
}

func (exchange *Exchange) LimitBuy(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeOrder("limit", "buy", amount, price, currency)
}

func (exchange *Exchange) LimitSell(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return exchange.placeOrder("limit", "sell", amount, price, currency)
}

func (exchange *Exchange) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	return exchange.placeOrder("market", "buy", amount, price, currency)
}

func (exchange *Exchange) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return exchange.placeOrder("market", "sell", amount, price, currency)
}

func (exchange *Exchange) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	params := url.Values{}
	apiuri := PRIVATE + "CancelOrder"
	params.Set("txid", orderId)

	var respmap map[string]interface{}
	err := exchange.doAuthenticatedRequest("POST", apiuri, params, &respmap)
	if err != nil {
		return false, err
	}
	//log.Println(respmap)
	return true, nil
}

func (exchange *Exchange) toOrder(orderinfo interface{}) Order {
	omap := orderinfo.(map[string]interface{})
	descmap := omap["descr"].(map[string]interface{})
	pair := descmap["pair"].(string)
	ind := strings.Index(pair, "EUR")
	curr := Currency{Symbol: pair[:ind], Desc: ""}
	fiat := Currency{Symbol: pair[ind:], Desc: ""}
	currency := CurrencyPair{CurrencyA: curr, CurrencyB: fiat}
	return Order{
		Amount:     ToFloat64(omap["vol"]),
		Price:      ToFloat64(descmap["price"]),
		DealAmount: ToFloat64(omap["vol_exec"]),
		AvgPrice:   ToFloat64(omap["price"]),
		Side:       AdaptTradeSide(descmap["type"].(string)),
		Status:     exchange.convertOrderStatus(omap["status"].(string)),
		Fee:        ToFloat64(omap["fee"]),
		OrderTime:  ToInt(omap["opentm"]),
		Currency:   currency,
	}
}

func (exchange *Exchange) toTrade(tradeinfo interface{}) Trade {
	tmap := tradeinfo.(map[string]interface{})
	descmap := tmap["trades"].(map[string]interface{})
	pair := descmap["pair"].(string)
	ind := strings.Index(pair, "EUR")
	curr := Currency{Symbol: pair[:ind], Desc: ""}
	fiat := Currency{Symbol: pair[ind:], Desc: ""}
	currency := CurrencyPair{CurrencyA: curr, CurrencyB: fiat}
	return Trade{
		Amount:    ToFloat64(descmap["vol"]),
		Price:     ToFloat64(descmap["price"]),
		OrderType: AdaptTradeSide(descmap["type"].(string)),
		Fee:       ToFloat64(descmap["fee"]),
		OrderTime: ToInt(descmap["time"]),
		Pair:      currency,
		OrderID2:  descmap["ordertxid"].(string),
		Tid:       ToInt64(tmap["txid"]),
	}
}

func (exchange *Exchange) GetOrderInfos(txids ...string) ([]Order, error) {
	params := url.Values{}
	params.Set("txid", strings.Join(txids, ","))

	var resultmap map[string]interface{}
	err := exchange.doAuthenticatedRequest("POST", PRIVATE+"QueryOrders", params, &resultmap)
	if err != nil {
		return nil, err
	}
	//log.Println(resultmap)
	var ords []Order
	for txid, v := range resultmap {
		ord := exchange.toOrder(v)
		ord.OrderID2 = txid
		ords = append(ords, ord)
	}

	return ords, nil
}

func (exchange *Exchange) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	orders, err := exchange.GetOrderInfos(orderId)

	if err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return nil, errors.New("not fund the order " + orderId)
	}

	ord := &orders[0]
	ord.Currency = currency
	return ord, nil
}

func (exchange *Exchange) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	var result struct {
		Open map[string]interface{} `json:"open"`
	}

	err := exchange.doAuthenticatedRequest("POST", PRIVATE+"OpenOrders", url.Values{}, &result)
	if err != nil {
		return nil, err
	}

	var orders []Order

	for txid, v := range result.Open {
		ord := exchange.toOrder(v)
		ord.OrderID2 = txid
		orders = append(orders, ord)
	}

	return orders, nil
}

func (exchange *Exchange) GetOrderHistorys(currency CurrencyPair, optional ...OptionalParameter) ([]Order, error) {
	panic("")
}

func (exchange *Exchange) GetTradeHistory(currency CurrencyPair, optional ...OptionalParameter) ([]Trade, error) {
	var resultmap map[string]map[string]interface{}

	err := exchange.doAuthenticatedRequest("POST", PRIVATE+"TradesHistory?ofs=0", url.Values{}, &resultmap)
	if err != nil {
		return nil, err
	}

	var allTrades []Trade
	for _, trade := range resultmap {
		singleTrade := exchange.toTrade(trade)
		allTrades = append(allTrades, singleTrade)
	}
	return allTrades, err
}

func (exchange *Exchange) GetAccount() (*Account, error) {
	params := url.Values{}
	apiuri := PRIVATE + "Balance"

	var resustmap map[string]interface{}
	err := exchange.doAuthenticatedRequest("POST", apiuri, params, &resustmap)
	if err != nil {
		return nil, err
	}

	acc := new(Account)
	acc.Exchange = exchange.GetExchangeName()
	acc.SubAccounts = make(map[Currency]SubAccount)

	for key, v := range resustmap {
		currency := exchange.convertCurrency(key)
		amount := ToFloat64(v)
		//log.Println(symbol, amount)
		acc.SubAccounts[currency] = SubAccount{Currency: currency, Amount: amount, ForzenAmount: 0, LoanAmount: 0}

		if currency.Symbol == "XBT" { // adapt to btc
			acc.SubAccounts[BTC] = SubAccount{Currency: BTC, Amount: amount, ForzenAmount: 0, LoanAmount: 0}
		}
	}

	return acc, nil

}

//func (exchange *Kraken) GetTradeBalance() {
//	var resultmap map[string]interface{}
//	k.doAuthenticatedRequest("POST", PRIVATE+"TradeBalance", url.Values{}, &resultmap)
//	log.Println(resultmap)
//}

// GetAssets will return the full list of currency pairs available
func (exchange *Exchange) GetAssets(currency CurrencyPair) (*Assets, error) {
	var resultmap map[string]map[string]interface{}
	assets := new(Assets)
	assetName := exchange.convertPair(currency).ToSymbol("")
	if assetName == "all_" {
		assetName = "all"
	}
	err := exchange.doAuthenticatedRequest("GET", PUBLIC+"AssetPairs?asset="+assetName, url.Values{}, &resultmap)
	if err != nil {
		return nil, err
	}

	for _, content := range resultmap {
		assets.Assets = append(assets.Assets, NewCurrencyPair3(fmt.Sprintf("%v", content["wsname"]), "/"))
	}
	return assets, err
}

func (exchange *Exchange) GetTicker(currency CurrencyPair) (*Ticker, error) {
	var resultmap map[string]interface{}
	err := exchange.doAuthenticatedRequest("GET", "public/Ticker?pair="+exchange.convertPair(currency).ToSymbol(""), url.Values{}, &resultmap)
	if err != nil {
		return nil, err
	}

	ticker := new(Ticker)
	ticker.Pair = currency
	for _, t := range resultmap {
		tickermap := t.(map[string]interface{})
		ticker.Last = ToFloat64(tickermap["c"].([]interface{})[0])
		ticker.Buy = ToFloat64(tickermap["b"].([]interface{})[0])
		ticker.Sell = ToFloat64(tickermap["a"].([]interface{})[0])
		ticker.Low = ToFloat64(tickermap["l"].([]interface{})[0])
		ticker.High = ToFloat64(tickermap["h"].([]interface{})[0])
		ticker.Vol = ToFloat64(tickermap["v"].([]interface{})[0])
	}

	return ticker, nil
}

func (exchange *Exchange) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	apiuri := fmt.Sprintf(PUBLIC+"Depth?pair=%s&count=%d", exchange.convertPair(currency).ToSymbol(""), size)
	var resultmap map[string]interface{}
	err := exchange.doAuthenticatedRequest("GET", apiuri, url.Values{}, &resultmap)
	if err != nil {
		return nil, err
	}

	//log.Println(respmap)
	dep := Depth{}
	dep.Pair = currency
	for _, d := range resultmap {
		depmap := d.(map[string]interface{})
		asksmap := depmap["asks"].([]interface{})
		bidsmap := depmap["bids"].([]interface{})
		for _, v := range asksmap {
			ask := v.([]interface{})
			dep.AskList = append(dep.AskList, DepthRecord{ToFloat64(ask[0]), ToFloat64(ask[1])})
		}
		for _, v := range bidsmap {
			bid := v.([]interface{})
			dep.BidList = append(dep.BidList, DepthRecord{ToFloat64(bid[0]), ToFloat64(bid[1])})
		}
		break
	}

	sort.Sort(sort.Reverse(dep.AskList)) //reverse

	return &dep, nil
}

func (exchange *Exchange) GetKlineRecords(currency CurrencyPair, period KlinePeriod, size int, opt ...OptionalParameter) ([]Kline, error) {
	panic("")
}

//非个人，整个交易所的交易记录
func (exchange *Exchange) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	panic("")
}

func (exchange *Exchange) GetExchangeName() string {
	return KRAKEN
}

func (exchange *Exchange) buildParamsSigned(apiuri string, postForm *url.Values) string {
	postForm.Set("nonce", fmt.Sprintf("%d", time.Now().UnixNano()))
	urlPath := API_V0 + apiuri

	secretByte, _ := base64.StdEncoding.DecodeString(exchange.secretKey)
	encode := []byte(postForm.Get("nonce") + postForm.Encode())

	sha := sha256.New()
	sha.Write(encode)
	shaSum := sha.Sum(nil)

	pathSha := append([]byte(urlPath), shaSum...)

	mac := hmac.New(sha512.New, secretByte)
	mac.Write(pathSha)
	macSum := mac.Sum(nil)

	sign := base64.StdEncoding.EncodeToString(macSum)

	return sign
}

func (exchange *Exchange) doAuthenticatedRequest(method, apiuri string, params url.Values, ret interface{}) error {
	headers := map[string]string{}

	if "POST" == method {
		signature := exchange.buildParamsSigned(apiuri, &params)
		headers = map[string]string{
			"API-Key":  exchange.accessKey,
			"API-Sign": signature,
		}
	}

	resp, err := NewHttpRequest(exchange.httpClient, method, API_DOMAIN+apiuri, params.Encode(), headers)
	if err != nil {
		return err
	}
	//println(string(resp))
	var base BaseResponse
	base.Result = ret

	err = json.Unmarshal(resp, &base)
	if err != nil {
		return err
	}

	//println(string(resp))

	if len(base.Error) > 0 {
		return errors.New(base.Error[0])
	}

	return nil
}

func (exchange *Exchange) convertCurrency(currencySymbol string) Currency {
	if len(currencySymbol) >= 4 {
		currencySymbol = strings.Replace(currencySymbol, "X", "", 1)
		currencySymbol = strings.Replace(currencySymbol, "Z", "", 1)
	}
	return NewCurrency(currencySymbol, "")
}

func (exchange *Exchange) convertPair(pair CurrencyPair) CurrencyPair {
	if "BTC" == pair.CurrencyA.Symbol {
		return NewCurrencyPair(XBT, pair.CurrencyB)
	}

	if "BTC" == pair.CurrencyB.Symbol {
		return NewCurrencyPair(pair.CurrencyA, XBT)
	}

	return pair
}

func (exchange *Exchange) convertOrderStatus(status string) TradeStatus {
	switch status {
	case "open", "pending":
		return ORDER_UNFINISH
	case "canceled", "expired":
		return ORDER_CANCEL
	case "filled", "closed":
		return ORDER_FINISH
	case "partialfilled":
		return ORDER_PART_FINISH
	}
	return ORDER_UNFINISH
}
