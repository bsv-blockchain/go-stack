package price

import (
	"fmt"

	"github.com/patrickmn/go-cache"
	woc_exchange_rate "github.com/teranode-group/proto/woc-exchange-rate"
)

type ExchangeRate struct {
	Rate     float64 `json:"rate,omitempty"`
	Time     *int64  `json:"time,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

var EXCHANGE_RATE_KEY = "EXCHANGE_RATE_KEY"

func GetUSDPrice() (float64, error) {

	exchangeRate := GetExchangeRateCache()

	if exchangeRate == nil {
		return 0, fmt.Errorf("failed to get USD price from cache")
	}
	return exchangeRate.Rate, nil
}

var exchangeRateCache = cache.New(cache.NoExpiration, cache.NoExpiration)

func SetExchangeRateCache(val *woc_exchange_rate.ExchangeRate) {
	exchangeRateCache.Set(EXCHANGE_RATE_KEY, val, cache.NoExpiration)
}

func GetExchangeRateCache() *woc_exchange_rate.ExchangeRate {
	ex, found := exchangeRateCache.Get(EXCHANGE_RATE_KEY)
	if found {
		exchangeRate := ex.(*woc_exchange_rate.ExchangeRate)
		return exchangeRate
	}
	return nil
}
