package huobi

import (
	"testing"

	"github.com/soulsplit/goex"
)

var wallet *Wallet

func init() {
	wallet = NewWallet(&goex.APIConfig{
		HttpClient:   httpProxyClient,
		ApiKey:       "",
		ApiSecretKey: "",
	})
}

func TestWallet_Transfer(t *testing.T) {
	t.Log(wallet.Transfer(goex.TransferParameter{
		Currency: "BTC",
		From:     goex.SWAP_USDT,
		To:       goex.SPOT,
		Amount:   11,
	}))
}
