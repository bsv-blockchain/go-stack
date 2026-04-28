package serverfiber

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	commongrpc "github.com/teranode-group/common/grpc"
	"github.com/teranode-group/common/logger"
	woc_exchange_rate "github.com/teranode-group/proto/woc-exchange-rate"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/price"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ExchangeRate struct {
	Rate     json.Number `json:"rate,omitempty"`
	Time     *int64      `json:"time,omitempty"`
	Currency string      `json:"currency,omitempty"`
}

type exchangeRateQuery struct {
	From   int64
	To     int64
	Period string
}

func (s *Server) LatestExchangeRate(c *fiber.Ctx) error {
	exchangeRate := price.GetExchangeRateCache()
	var wocExchangeRateRes *woc_exchange_rate.GetLatestExchangeRateResponse
	var err error

	if exchangeRate == nil {
		wocExchangeRateReq := &woc_exchange_rate.GetLatestExchangeRateRequest{
			Coin: woc_exchange_rate.CoinType_BSV,
		}
		wocExchangeRateRes, err = s.wocExchangeRateClient.GetLatestExchangeRate(c.UserContext(), wocExchangeRateReq)
		if err != nil {
			return fmt.Errorf("failed to get latest exchange rate: %w", err)
		}
		exchangeRate = wocExchangeRateRes.ExchangeRate
		price.SetExchangeRateCache(exchangeRate)
	}

	exchangeRateJson := &ExchangeRate{
		Rate:     json.Number(strconv.FormatFloat(exchangeRate.Rate, 'f', -1, 64)),
		Time:     &exchangeRate.Time.Seconds,
		Currency: "USD",
	}

	return c.JSON(exchangeRateJson)

}

func (s *Server) ExchangeRate(c *fiber.Ctx) error {
	var query exchangeRateQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	period, err := time.ParseDuration(query.Period)
	if err != nil {
		return fmt.Errorf("invalid period duration: %w", err)
	}

	wocExchangeRateRes, err := s.wocExchangeRateClient.GetExchangeRate(c.UserContext(), &woc_exchange_rate.GetExchangeRateRequest{
		From:   timestamppb.New(time.Unix(query.From, 0)),
		To:     timestamppb.New(time.Unix(query.To, 0)),
		Period: durationpb.New(period),
	})
	if err != nil {
		return fmt.Errorf("failed to get exchange rate: %w", err)
	}
	res := make([]ExchangeRate, len(wocExchangeRateRes.ExchangeRates))

	for k, rate := range wocExchangeRateRes.ExchangeRates {
		res[k] = ExchangeRate{
			Rate: json.Number(strconv.FormatFloat(rate.Rate, 'f', -1, 64)),
			Time: &rate.Time.Seconds,
		}
	}

	return c.JSON(res)
}

func (s *Server) GetExchangeRate(c context.Context, query exchangeRateQuery) ([]ExchangeRate, error) {

	period, err := time.ParseDuration(query.Period)
	if err != nil {
		return nil, fmt.Errorf("invalid period duration: %w", err)
	}

	wocExchangeRateRes, err := s.wocExchangeRateClient.GetExchangeRate(c, &woc_exchange_rate.GetExchangeRateRequest{
		From:   timestamppb.New(time.Unix(query.From, 0)),
		To:     timestamppb.New(time.Unix(query.To, 0)),
		Period: durationpb.New(period),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rate: %w", err)
	}
	res := make([]ExchangeRate, len(wocExchangeRateRes.ExchangeRates))

	for k, rate := range wocExchangeRateRes.ExchangeRates {
		res[k] = ExchangeRate{
			Rate: json.Number(strconv.FormatFloat(rate.Rate, 'f', -1, 64)),
			Time: &rate.Time.Seconds,
		}
	}

	return res, nil
}

func (s *Server) HistoricalExchangeRate(c *fiber.Ctx) error {
	var query exchangeRateQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	wocExchangeRateRes, err := s.wocExchangeRateClient.GetHistoricalExchangeRate(c.UserContext(), &woc_exchange_rate.GetExchangeRateRequest{
		From: timestamppb.New(time.Unix(query.From, 0)),
		To:   timestamppb.New(time.Unix(query.To, 0)),
	})
	if err != nil {
		return fmt.Errorf("failed to get historical exchange rate: %w", err)
	}
	res := make([]ExchangeRate, len(wocExchangeRateRes.ExchangeRates))

	for k, rate := range wocExchangeRateRes.ExchangeRates {
		res[k] = ExchangeRate{
			Rate: json.Number(strconv.FormatFloat(rate.Rate, 'f', -1, 64)),
			Time: &rate.Time.Seconds,
		}
	}

	return c.JSON(res)
}

func WocExchangeRateHealthCheck(ctx context.Context) (*woc_exchange_rate.HealthCheckResponse, error) {
	wocExchangeRateConnection, err := commongrpc.NewClientConnection(configs.Settings.WocExchangeRateAddress, logger.Log, nil)
	if err != nil {
		logger.Log.Info("failed to connect to woc-exchange-rate service", zap.Error(err))
		return &woc_exchange_rate.HealthCheckResponse{}, fmt.Errorf("failed to get connect to woc-exchange-rate service: %w", err)
	}

	defer wocExchangeRateConnection.Close()

	healthCheckClient := woc_exchange_rate.NewHealthClient(wocExchangeRateConnection)

	wocExchangeRateHealthResponse, err := healthCheckClient.Check(
		ctx,
		&woc_exchange_rate.HealthCheckRequest{
			Service: "woc-exchange-rate",
		},
	)

	if err != nil {
		logger.Log.Info("failed to get exchange rate health check:", zap.Error(err))
		return &woc_exchange_rate.HealthCheckResponse{}, fmt.Errorf("failed to get exchange rate health check: %w", err)
	}

	return wocExchangeRateHealthResponse, nil
}
