package okex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	. "github.com/soulsplit/goex"
	"github.com/soulsplit/goex/internal/logger"
)

const baseUrl = "https://www.okex.com"

type Exchange struct {
	config          *APIConfig
	OKExSpot        *OKExSpot
	OKExFuture      *OKExFuture
	OKExSwap        *OKExSwap
	OKExWallet      *OKExWallet
	OKExMargin      *ExchangeMargin
	OKExV3FuturesWs *OKExV3FuturesWs
	OKExV3SpotWs    *OKExV3SpotWs
	OKExV3SwapWs    *OKExV3SwapWs
}

func NewOKEx(config *APIConfig) *Exchange {
	if config.Endpoint == "" {
		config.Endpoint = baseUrl
	}
	okex := &Exchange{config: config}
	okex.OKExSpot = &OKExSpot{okex}
	okex.OKExFuture = &OKExFuture{Exchange: okex, Locker: new(sync.Mutex)}
	okex.OKExWallet = &OKExWallet{okex}
	okex.OKExMargin = &ExchangeMargin{okex}
	okex.OKExSwap = &OKExSwap{okex, config}
	okex.OKExV3FuturesWs = NewOKExV3FuturesWs(okex)
	okex.OKExV3SpotWs = NewOKExSpotV3Ws(okex)
	okex.OKExV3SwapWs = NewOKExV3SwapWs(okex)
	return okex
}

func (ok *Exchange) GetExchangeName() string {
	return OKEX
}

func (ok *Exchange) UUID() string {
	return strings.Replace(uuid.New().String(), "-", "", 32)
}

func (ok *Exchange) DoRequest(httpMethod, uri, reqBody string, response interface{}) error {
	url := ok.config.Endpoint + uri
	sign, timestamp := ok.doParamSign(httpMethod, uri, reqBody)
	//logger.Log.Debug("timestamp=", timestamp, ", sign=", sign)
	resp, err := NewHttpRequest(ok.config.HttpClient, httpMethod, url, reqBody, map[string]string{
		CONTENT_TYPE: APPLICATION_JSON_UTF8,
		ACCEPT:       APPLICATION_JSON,
		//COOKIE:               LOCALE + "en_US",
		OK_ACCESS_KEY:        ok.config.ApiKey,
		OK_ACCESS_PASSPHRASE: ok.config.ApiPassphrase,
		OK_ACCESS_SIGN:       sign,
		OK_ACCESS_TIMESTAMP:  fmt.Sprint(timestamp)})
	if err != nil {
		//log.Println(err)
		return err
	} else {
		logger.Log.Debug(string(resp))
		return json.Unmarshal(resp, &response)
	}
}

func (ok *Exchange) adaptOrderState(state int) TradeStatus {
	switch state {
	case -2:
		return ORDER_FAIL
	case -1:
		return ORDER_CANCEL
	case 0:
		return ORDER_UNFINISH
	case 1:
		return ORDER_PART_FINISH
	case 2:
		return ORDER_FINISH
	case 3:
		return ORDER_UNFINISH
	case 4:
		return ORDER_CANCEL_ING
	}
	return ORDER_UNFINISH
}

/*
 Get a http request body is a json string and a byte array.
*/
func (ok *Exchange) BuildRequestBody(params interface{}) (string, *bytes.Reader, error) {
	if params == nil {
		return "", nil, errors.New("illegal parameter")
	}
	data, err := json.Marshal(params)
	if err != nil {
		//log.Println(err)
		return "", nil, errors.New("json convert string error")
	}

	jsonBody := string(data)
	binBody := bytes.NewReader(data)

	return jsonBody, binBody, nil
}

func (ok *Exchange) doParamSign(httpMethod, uri, requestBody string) (string, string) {
	timestamp := ok.IsoTime()
	preText := fmt.Sprintf("%s%s%s%s", timestamp, strings.ToUpper(httpMethod), uri, requestBody)
	//log.Println("preHash", preText)
	sign, _ := GetParamHmacSHA256Base64Sign(ok.config.ApiSecretKey, preText)
	return sign, timestamp
}

/*
 Get a iso time
  eg: 2018-03-16T18:02:48.284Z
*/
func (ok *Exchange) IsoTime() string {
	utcTime := time.Now().UTC()
	iso := utcTime.String()
	isoBytes := []byte(iso)
	iso = string(isoBytes[:10]) + "T" + string(isoBytes[11:23]) + "Z"
	return iso
}

func (ok *Exchange) LimitBuy(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return ok.OKExSpot.LimitBuy(amount, price, currency, opt...)
}

func (ok *Exchange) LimitSell(amount, price string, currency CurrencyPair, opt ...LimitOrderOptionalParameter) (*Order, error) {
	return ok.OKExSpot.LimitSell(amount, price, currency, opt...)
}

func (ok *Exchange) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	return ok.OKExSpot.MarketBuy(amount, price, currency)
}

func (ok *Exchange) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	return ok.OKExSpot.MarketSell(amount, price, currency)
}

func (ok *Exchange) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	return ok.OKExSpot.OKExSpot.CancelOrder(orderId, currency)
}

func (ok *Exchange) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	return ok.OKExSpot.GetOneOrder(orderId, currency)
}

func (ok *Exchange) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	return ok.OKExSpot.GetUnfinishOrders(currency)
}

func (ok *Exchange) GetOrderHistorys(currency CurrencyPair, opt ...OptionalParameter) ([]Order, error) {
	return ok.OKExSpot.GetOrderHistorys(currency, opt...)
}

func (ok *Exchange) GetAccount() (*Account, error) {
	return ok.OKExSpot.GetAccount()
}

func (ok *Exchange) GetTicker(currency CurrencyPair) (*Ticker, error) {
	return ok.OKExSpot.GetTicker(currency)
}

func (ok *Exchange) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	return ok.OKExSpot.GetDepth(size, currency)
}

func (ok *Exchange) GetKlineRecords(currency CurrencyPair, period KlinePeriod, size int, optional ...OptionalParameter) ([]Kline, error) {
	return ok.OKExSpot.GetKlineRecords(currency, period, size, optional...)
}

func (ok *Exchange) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
	return ok.OKExSpot.GetTrades(currencyPair, since)
}

func (ok *Exchange) GetAssets(currency CurrencyPair) (*Assets, error) {
	panic("")
}

func (exchange *Exchange) GetTradeHistory(currency CurrencyPair, optional ...OptionalParameter) ([]Trade, error) {
	panic("")
}
