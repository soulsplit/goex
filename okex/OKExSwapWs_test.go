package okex

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/soulsplit/goex"
	"github.com/soulsplit/goex/internal/logger"
)

func init() {
	logger.SetLevel(logger.DEBUG)
}

func TestNewOKExV3SwapWs(t *testing.T) {
	os.Setenv("HTTPS_PROXY", "socks5://127.0.0.1:1080")
	ok := NewOKEx(&goex.APIConfig{
		HttpClient: http.DefaultClient,
	})
	ok.OKExV3SwapWs.TickerCallback(func(ticker *goex.FutureTicker) {
		t.Log(ticker.Ticker, ticker.ContractType)
	})
	ok.OKExV3SwapWs.DepthCallback(func(depth *goex.Depth) {
		t.Log(depth)
	})
	ok.OKExV3SwapWs.TradeCallback(func(trade *goex.Trade, s string) {
		t.Log(s, trade)
	})
	ok.OKExV3SwapWs.SubscribeTicker(goex.BTC_USDT, goex.SWAP_CONTRACT)
	time.Sleep(1 * time.Minute)
}
