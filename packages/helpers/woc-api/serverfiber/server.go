package serverfiber

import (
	"context"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/ordishs/gocore"
	"github.com/teranode-group/common/logger"
	p2p_service "github.com/teranode-group/proto/p2p-service"
	woc_exchange_rate "github.com/teranode-group/proto/woc-exchange-rate"
	woc_stats "github.com/teranode-group/proto/woc-stats"
	"github.com/teranode-group/woc-api/bitcoin"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/internal"
	"github.com/teranode-group/woc-api/price"
	"github.com/teranode-group/woc-api/redis"
	"go.uber.org/zap"
)

var bitcoinClient *bitcoin.Client
var isMainnet bool
var network string

type Server struct {
	app                   *fiber.App
	wocStatsClient        woc_stats.WocStatsClient
	wocExchangeRateClient woc_exchange_rate.WocExchangeRateClient
	p2pServiceClient      p2p_service.P2PServiceClient
}

const (
	KEY_GetTagsSummaryFromDay        = "TagsSummary/1"
	KEY_GetTagsSummaryFromSevenDays  = "TagsSummary/7"
	KEY_GetTagsSummaryFromThirtyDays = "TagsSummary/30"
	KEY_GetTagsSummaryFromNinetyDays = "TagsSummary/90"
	KEY_GetTagsSummaryFromOneYear    = "TagsSummary/365"
	KEY_GetTagsSummaryPerWeek        = "TagsSummaryPerWeek"
	//As we want to limit the amount of data coming back we set the max number of days that
	//we want to get for tags
	MAX_TAGS_DAYS = 1223

	KEY_TxCountForLast24Hours             = "TxCountForLast24Hours"
	KEY_ExchangeRateForLast24Hours        = "ExchangeRateForLast24Hours"
	KEY_MinerTagStatsForLast24Hours       = "MinerTagStatsForLast24Hours"
	KEY_LastHomePageStatsUpdateFor24Hours = "lastHomePageStatsUpdateFor24Hours"
)

var TagsSummaryCacheKeys = map[int]string{
	1:             KEY_GetTagsSummaryFromDay,
	7:             KEY_GetTagsSummaryFromSevenDays,
	30:            KEY_GetTagsSummaryFromThirtyDays,
	90:            KEY_GetTagsSummaryFromNinetyDays,
	365:           KEY_GetTagsSummaryFromOneYear,
	MAX_TAGS_DAYS: KEY_GetTagsSummaryPerWeek,
}

func New(wocStatsClient woc_stats.WocStatsClient, wocExchangeRateClient woc_exchange_rate.WocExchangeRateClient, p2pServiceClient p2p_service.P2PServiceClient) *Server {
	app := fiber.New(fiber.Config{
		JSONEncoder:           json.Marshal,   // faster marshal // SEE: library being used above
		JSONDecoder:           json.Unmarshal, // faster unmarshal // SEE: library being used above
		DisableStartupMessage: true,
		Prefork:               false, // To enable when heavier endpoints start using fiber: SEE: https://github.com/gofiber/fiber/issues/180#issuecomment-590009242
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Log.Info("endpoint error", zap.String("URI", c.Request().URI().String()), zap.Error(err))

			return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
		},
	})
	app.Use(compress.New())
	app.Use(recover.New())
	app.Use(requestid.New())

	return &Server{
		app:                   app,
		wocStatsClient:        wocStatsClient,
		wocExchangeRateClient: wocExchangeRateClient,
		p2pServiceClient:      p2pServiceClient,
	}
}

func (s *Server) Start() error {
	var err error
	bitcoinClient, err = bitcoin.New()
	if err != nil {
		logger.Log.Error("unable to create bitcoin client: %w", zap.Error(err))
	}

	isMainnet = gocore.Config().GetBool("isMainnet", true)
	network, _ = gocore.Config().Get("network")

	s.createRoutes()

	if configs.Settings.BlockHeadersSaveLatestEnabled {
		_, _, err = s.GetLatestBlockHeaders(0)
		if err != nil {
			logger.Log.Error("unable to get latest block headers", zap.Error(err))
		}

		ticker2 := time.NewTicker(30 * time.Second)

		go func() {
			isbusy := false
			for range ticker2.C {
				if !isbusy {
					isbusy = true
					_, _, err := s.GetLatestBlockHeaders(0)
					if err != nil {
						logger.Log.Error("unable to get latest block headers", zap.Error(err))
					}
					isbusy = false
				}
			}
		}()
	}

	if configs.Settings.BlockHeadersSaveEnabled {
		// run once right away
		if err := internal.SaveBlockHeaders(); err != nil {
			logger.Log.Error("failed to write block headers to file", zap.Error(err))
		}

		ticker3 := time.NewTicker(time.Duration(configs.Settings.BlockHeadersSaveToFileTimer) * time.Second)
		go func() {
			isbusy := false
			for range ticker3.C {
				if !isbusy {
					isbusy = true
					err := internal.SaveBlockHeaders()
					if err != nil {
						logger.Log.Error("failed to write block headers to file", zap.Error(err))
					}
					isbusy = false
				}
			}
		}()
	}

	if isMainnet {

		//start ExchangeRateCache
		if configs.Settings.WocExchangeRateEnabled {
			s.setExchangeRateCache()
			ticker1 := time.NewTicker(10 * time.Second)
			go func() {
				for range ticker1.C {
					s.setExchangeRateCache()
				}
			}()
		}

		//start TagSummaryCache
		go func() {
			s.setTagSummaryCache()
		}()

		//start HomePageStatsCache
		if configs.Settings.HomePageStatsCacheEnabled {
			go func() {
				s.setHomePageStatsCache()
			}()
		}

	}

	return s.app.Listen(fmt.Sprintf(":%d", configs.Settings.FiberPort))
}

func (s *Server) setExchangeRateCache() {
	ctx, cancel := context.WithCancel(context.Background())
	wocExchangeRateReq := &woc_exchange_rate.GetLatestExchangeRateRequest{
		Coin: woc_exchange_rate.CoinType_BSV,
	}
	wocExchangeRateRes, err := s.wocExchangeRateClient.GetLatestExchangeRate(ctx, wocExchangeRateReq)
	if err != nil {
		logger.Log.Error("failed to get latest exchange rate: %w", zap.Error(err))
	} else {
		price.SetExchangeRateCache(wocExchangeRateRes.ExchangeRate)
	}
	cancel()
}

func (s *Server) setTagSummaryCache() {
	if !redis.RedisClient.Enabled || !configs.Settings.WocStatsEnabled {
		return
	}

	logger.Log.Info("Starting Perodic Caching for tags summary data")

	ticker := time.NewTicker(30 * time.Minute)
	for ; true; <-ticker.C {
		ctx, cancel := context.WithCancel(context.Background())
		conn := redis.RedisClient.ConnPool.Get()
		for days, key := range TagsSummaryCacheKeys {
			data, err := s.GetTagsSummary(ctx, days)
			if err != nil {
				logger.Log.Error("failed to fetch tags summary for %s days: %w", zap.Int("days", days), zap.Error(err))
			}
			if redis.RedisClient.Enabled {
				err = redis.SetCacheValue(key, data, conn)
				if err != nil {
					logger.Log.Error("failed to cache tags summary for %s days: %w", zap.Int("days", days), zap.Error(err))
				}
			}
		}
		cancel()
		conn.Flush()
		conn.Close()
	}
}

func (s *Server) setHomePageStatsCache() {
	if !redis.RedisClient.Enabled || !configs.Settings.WocStatsEnabled {
		return
	}

	logger.Log.Info("Starting Perodic Caching for home page charts data")

	cacheExpiry := int64(configs.Settings.HomePageStatsCacheExpiry)

	ticker := time.NewTicker(10 * time.Second)
	for ; true; <-ticker.C {
		ctx, cancel := context.WithCancel(context.Background())
		conn := redis.RedisClient.ConnPool.Get()

		data, err := s.GetTagsSummary(ctx, 1)
		if err != nil {
			logger.Log.Error("failed to fetch tags summary for 1 day: %w", zap.Error(err))
		}

		// day expiry
		err = redis.SetCacheValueWithExpire(KEY_GetTagsSummaryFromDay, data, cacheExpiry, conn)
		if err != nil {
			logger.Log.Error("failed to cache tags summary for 1 day: %w", zap.Error(err))
		}

		// to and from over the last 24 hours
		to := time.Now().Unix()
		from := time.Now().Add(-24 * time.Hour).Unix()

		//Store tx count over the last 24 hours

		fields := []string{"header.block_size", "header.time", "header.height", "header.tx_count"}

		statsQuery := statsBlockQuery{
			From:   from,
			To:     to,
			Fields: fields,
		}

		blockStatsdata, err := s.GetBlockStats(ctx, statsQuery)
		if err != nil {
			logger.Log.Error("failed to block stats for last 24 hours: %w", zap.Error(err))
		}

		err = redis.SetCacheValueWithExpire(KEY_TxCountForLast24Hours, blockStatsdata, cacheExpiry, conn)
		if err != nil {
			logger.Log.Error("failed to cache block stats for last 24 hours: %w", zap.Error(err))
		}

		//Store Miner tag stats over the last 24 hours

		fields = []string{"stats.total_size", "stats.total_fee", "header.size", "header.time", "header.height", "stats.total_tx_count", "details.miner_tag"}

		minerStatsBlockQuery := statsBlockQuery{
			From:   from,
			To:     to,
			Fields: fields,
		}

		minerTagBlockStatsData, err := s.GetBlockStats(ctx, minerStatsBlockQuery)
		if err != nil {
			logger.Log.Error("failed to block stats for last 24 hours: %w", zap.Error(err))
		}

		err = redis.SetCacheValueWithExpire(KEY_MinerTagStatsForLast24Hours, minerTagBlockStatsData, cacheExpiry, conn)
		if err != nil {
			logger.Log.Error("failed to cache block stats for last 24 hours: %w", zap.Error(err))
		}

		//Store exchange rate data over the last 24 hours

		exchangeRateQuery := exchangeRateQuery{
			From:   from,
			To:     to,
			Period: "5m",
		}

		exchangeRatedata, err := s.GetExchangeRate(ctx, exchangeRateQuery)
		if err != nil {
			logger.Log.Error("failed to fetch exchange rate data over the last 24 hours: %w", zap.Error(err))
		}

		err = redis.SetCacheValueWithExpire(KEY_ExchangeRateForLast24Hours, exchangeRatedata, cacheExpiry, conn)
		if err != nil {
			logger.Log.Error("failed to cache exchange rate data over the last 24 hours: %w", zap.Error(err))
		}

		lastUpdate := time.Now().Unix()

		err = redis.SetCacheValueWithExpire(KEY_LastHomePageStatsUpdateFor24Hours, lastUpdate, cacheExpiry, conn)
		if err != nil {
			logger.Log.Error("failed to last update time over the last 24 hours: %w", zap.Error(err))
		}

		cancel()
		conn.Flush()
		conn.Close()
	}
}

func SetBitcoinNetwork(net bool) {
	isMainnet = net
}
