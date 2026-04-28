package serverfiber

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/price"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/iancoleman/strcase"
	fieldmask_utils "github.com/mennanov/fieldmask-utils"
	"github.com/ordishs/gocore"
	"github.com/teranode-group/common/logger"
	woc_exchange_rate "github.com/teranode-group/proto/woc-exchange-rate"
	woc_stats "github.com/teranode-group/proto/woc-stats"
	"github.com/teranode-group/woc-api/search"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type BlockHeader struct {
	Hash            *string                `json:"hash,omitempty"`
	TxCount         *int64                 `json:"tx_count,omitempty"`
	Size            *int64                 `json:"total_size,omitempty"`
	Height          *uint64                `json:"height,omitempty"`
	Version         *uint64                `json:"version,omitempty"`
	VersionHex      *string                `json:"version_hex,omitempty"`
	MerkleRoot      *string                `json:"merkle_root,omitempty"`
	Time            *timestamppb.Timestamp `json:"-"`
	TimeUnix        *int64                 `json:"time,omitempty"`
	MedianTime      *uint64                `json:"median_time,omitempty"`
	Nonce           *uint64                `json:"nonce,omitempty"`
	Bits            *string                `json:"bits,omitempty"`
	Difficulty      *float64               `json:"difficulty,omitempty"`
	Chainwork       *string                `json:"chainwork,omitempty"`
	MinerTag        *string                `json:"miner_tag,omitempty"`
	TotalFee        *int64                 `json:"total_fee,omitempty"`
	TotaFeeInUsd    json.Number            `json:"total_fee_usd,omitempty"`
	TotalOut        *int64                 `json:"total_out,omitempty"`
	AvgFee          *int64                 `json:"avg_fee,omitempty"`
	AvergaeFeeInUsd json.Number            `json:"avg_fee_usd,omitempty"`
	MedianFee       *int64                 `json:"median_fee,omitempty"`
	MedianFeeInUsd  json.Number            `json:"median_fee_usd,omitempty"`
	InputCount      *int64                 `json:"input_count,omitempty"`
	OutputCount     *int64                 `json:"output_count,omitempty"`
	TotalOutInUsd   json.Number            `json:"total_out_usd,omitempty"`
	Subsidy         *int64                 `json:"subsidy,omitempty"`
	SubsidyInUsd    json.Number            `json:"subsidy_usd,omitempty"`
	Reward          *int64                 `json:"reward,omitempty"`
	RewardInUsd     json.Number            `json:"reward_usd,omitempty"`
	Coinbase        *string                `json:"coinbase,omitempty"`
	BlockSize       *uint64                `json:"size,omitempty"`
	ExchangeRate    ExchangeRate           `json:"exchange_rate,omitempty"`
	MinTxSize       *int64                 `json:"min_tx_size,omitempty"`
	MaxTxSize       *int64                 `json:"max_tx_size,omitempty"`
	MedianTxSize    *int64                 `json:"median_tx_size,omitempty"`
	AvgTxSize       *int64                 `json:"avg_tx_size,omitempty"`
	CddTotal        *float64               `json:"cdd_total,omitempty"`
}

func (s *Server) Block(c *fiber.Ctx) error {

	var query string

	if c.Params("height") != "" {
		query = c.Params("height")
	}

	if c.Params("hash") != "" {
		query = c.Params("hash")
	}

	var wocBlockRes = &woc_stats.GetBlockHeaderResponse{}

	if len(query) == 64 {
		var err error
		wocBlockRes, err = s.wocStatsClient.GetBlockByHash(c.UserContext(), &woc_stats.GetBlockByHashRequest{
			Hash: query,
		})
		if err != nil {
			return fmt.Errorf("failed to get block by hash from woc stats: %w", err)
		}

	} else {
		height, err := strconv.ParseInt(query, 10, 64)

		if err != nil {
			return fmt.Errorf("invalid height query: %w", err)
		}

		wocBlockRes, err = s.wocStatsClient.GetBlockByHeight(c.UserContext(), &woc_stats.GetBlockByHeightRequest{
			Height: height,
		})

		if err != nil {
			return fmt.Errorf("failed to get block by height from woc stats: %w", err)
		}
	}

	b := setBlockHeader(wocBlockRes.Header)

	err := setExchangeRate(&b, s, c)
	if err != nil {
		logger.Log.Error("Exchnage rate failure", zap.Error(err))
	}

	return c.JSON(b)

}

func (s *Server) BlocksByHeightRange(c *fiber.Ctx) error {

	fromParam := c.Params("from")
	toParam := c.Params("to")

	from, err := strconv.ParseInt(fromParam, 10, 64)

	if err != nil {
		return fmt.Errorf("invalid From param: %w", err)
	}

	to, err := strconv.ParseInt(toParam, 10, 64)

	if err != nil {
		return fmt.Errorf("invalid To param: %w", err)
	}

	wocBlocksRes, err := s.wocStatsClient.GetBlockByHeightRange(c.UserContext(), &woc_stats.GetBlockByHeightRangeRequest{
		From: from,
		To:   to,
	})
	if err != nil {
		return fmt.Errorf("failed to get blocks from woc stats: %w", err)
	}

	res := make([]BlockHeader, len(wocBlocksRes.Headers))

	for k, block := range wocBlocksRes.Headers {
		b := setBlockHeader(block)

		err := setExchangeRate(&b, s, c)
		if err != nil {
			logger.Log.Error("Exchnage rate failure", zap.Error(err))
		}

		res[k] = b
	}

	return c.JSON(res)

}

func setBlockUsdVals(rate float64, b *BlockHeader) {
	totalOutInUsd := float64(*b.TotalOut) * (rate / 1e8)
	b.TotalOutInUsd = json.Number(strconv.FormatFloat(totalOutInUsd, 'f', -1, 64))

	totalFeeInUsd := float64(*b.TotalFee) * (rate / 1e8)
	b.TotaFeeInUsd = json.Number(strconv.FormatFloat(totalFeeInUsd, 'f', -1, 64))

	subsidyInUsd := float64(*b.Subsidy) * (rate / 1e8)
	b.SubsidyInUsd = json.Number(strconv.FormatFloat(subsidyInUsd, 'f', -1, 64))

	rewardInUsd := float64(*b.Reward) * (rate / 1e8)
	b.RewardInUsd = json.Number(strconv.FormatFloat(rewardInUsd, 'f', -1, 64))

	medianFeeInUsd := float64(*b.MedianFee) * (rate / 1e8)
	b.MedianFeeInUsd = json.Number(strconv.FormatFloat(medianFeeInUsd, 'f', -1, 64))

	avgFeeInUsd := float64(*b.AvgFee) * (rate / 1e8)
	b.AvergaeFeeInUsd = json.Number(strconv.FormatFloat(avgFeeInUsd, 'f', -1, 64))
}

func setBlockHeader(wocBlockRes *woc_stats.BlockHeader) BlockHeader {
	var b BlockHeader

	b.Hash = &wocBlockRes.Hash
	b.TxCount = &wocBlockRes.TxCount
	b.BlockSize = &wocBlockRes.BlockSize
	b.Version = &wocBlockRes.Version
	b.VersionHex = &wocBlockRes.VersionHex
	b.MerkleRoot = &wocBlockRes.MerkleRoot
	b.Height = &wocBlockRes.Height
	b.MedianTime = &wocBlockRes.MedianTime
	b.Nonce = &wocBlockRes.Nonce
	b.Bits = &wocBlockRes.Bits
	b.Coinbase = &wocBlockRes.Coinbase
	b.Difficulty = &wocBlockRes.Difficulty
	b.Chainwork = &wocBlockRes.Chainwork
	b.MinerTag = &wocBlockRes.MinerTag
	b.TotalFee = &wocBlockRes.TotalFee
	b.AvgFee = &wocBlockRes.AvgFee
	b.Time = wocBlockRes.Time
	if b.Time != nil {
		b.TimeUnix = &b.Time.Seconds
	}
	b.OutputCount = &wocBlockRes.Outs
	b.InputCount = &wocBlockRes.Ins
	b.Subsidy = &wocBlockRes.Subsidy

	var reward int64 = wocBlockRes.Subsidy + wocBlockRes.TotalFee
	b.Reward = &reward
	b.TotalOut = &wocBlockRes.TotalOut
	b.MinTxSize = &wocBlockRes.MinTxSize
	b.MaxTxSize = &wocBlockRes.MaxTxSize
	b.MedianTxSize = &wocBlockRes.MedianTxSize
	b.AvgTxSize = &wocBlockRes.AvgTxSize
	b.MedianFee = &wocBlockRes.MedianFee

	if wocBlockRes.CddTotal > -1 {
		b.CddTotal = &wocBlockRes.CddTotal
	}

	return b
}

func setExchangeRate(b *BlockHeader, s *Server, c *fiber.Ctx) error {
	currentTime := time.Now().UTC()
	blockTime := time.Unix(b.Time.Seconds, 0)

	days := int(currentTime.Sub(blockTime).Hours() / 24)

	var wocExchangeRateRes *woc_exchange_rate.GetExchangeRateResponse
	var err error

	if days > 1 {
		wocExchangeRateRes, err = s.wocExchangeRateClient.GetHistoricalExchangeRate(c.UserContext(), &woc_exchange_rate.GetExchangeRateRequest{
			From: timestamppb.New(blockTime),
			To:   timestamppb.New(blockTime),
		})
		if err != nil {
			return fmt.Errorf("failed to get exchange rate: %w", err)
		}
	} else {
		period, err := time.ParseDuration("5m")
		if err != nil {
			return fmt.Errorf("invalid period duration: %w", err)
		}
		wocExchangeRateRes, err = s.wocExchangeRateClient.GetExchangeRate(c.UserContext(), &woc_exchange_rate.GetExchangeRateRequest{
			From:   timestamppb.New(blockTime.AddDate(0, 0, -1)),
			To:     timestamppb.New(blockTime),
			Period: durationpb.New(period),
		})
		if err != nil {
			return fmt.Errorf("failed to get exchange rate: %w", err)
		}
	}

	if len(wocExchangeRateRes.ExchangeRates) > 0 {
		data := wocExchangeRateRes.ExchangeRates[len(wocExchangeRateRes.ExchangeRates)-1]
		setBlockUsdVals(data.Rate, b)
		b.ExchangeRate.Rate = json.Number(strconv.FormatFloat(data.Rate, 'f', -1, 64))
		b.ExchangeRate.Time = &data.Time.Seconds
	}
	return nil
}

type statskMinerQuery struct {
	Days int64 `query:"days"`
	From int64 `query:"from"`
	To   int64 `query:"to"`
}

func (s *Server) StatsBlockMiner(c *fiber.Ctx) error {

	var query statskMinerQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	if query.Days == 0 && (query.To == 0 && query.From == 0) {
		return c.Status(fiber.StatusBadRequest).SendString("Valid query not provided")
	}

	var fields = []string{"stats.total_size", "stats.total_fee", "header.size", "header.time", "header.height", "details.miner_tag"}

	fieldMask, err := fieldmaskpb.New(&woc_stats.Block{}, fields...)
	if err != nil {
		return fmt.Errorf("invalid fields: %w", err)
	}

	orderBy := make([]*woc_stats.OrderBy, 0)

	var from time.Time
	var to time.Time

	if query.Days != 0 {
		if query.Days != 1 && query.Days != 30 {
			return c.Status(fiber.StatusBadRequest).SendString("Days query must either be 1 or 30")
		}

		current := time.Now().UTC()
		if query.Days > 1 {
			current = time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, current.Location())
		}
		days := int(query.Days * -1)

		from = current.AddDate(0, 0, days)
		to = current

		if query.Days > 1 {
			to = current.AddDate(0, 0, 1)
		}
	} else {
		if query.To == 0 || query.From == 0 {
			return c.Status(fiber.StatusBadRequest).SendString("From and To query must be provided")
		}
		from = time.Unix(query.From, 0)
		to = time.Unix(query.To, 0)
		difference := to.Sub(from)
		if int64(difference.Hours()/24) > 30 {
			return c.Status(fiber.StatusBadRequest).SendString("From date is greater then 30 days")
		}
	}

	wocStatsRes, err := s.wocStatsClient.GetBlock(c.UserContext(), &woc_stats.GetBlockRequest{
		From:      timestamppb.New(from),
		To:        timestamppb.New(to),
		FieldMask: fieldMask,
		OrderBy:   orderBy,
	})
	if err != nil {
		return fmt.Errorf("failed to get block from woc stats: %w", err)
	}

	mask, err := fieldmask_utils.MaskFromPaths(fields, strcase.ToCamel)
	if err != nil {
		return fmt.Errorf("failed to create a mask from paths: %w", err)
	}

	res := make([]Block, len(wocStatsRes.Blocks))

	if err != nil {
		logger.Log.Error("Exchnage rate failure", zap.Error(err))
	}

	for k, block := range wocStatsRes.Blocks {
		var b Block

		// using local BlockStats so that it has nil fields and return when on zero-value
		if err = fieldmask_utils.StructToStruct(mask, block, &b); err != nil {
			return fmt.Errorf("failed to create struct from field map: %w", err)
		}

		if b.Header.Time != nil { // nolint: staticcheck // not sure why lint failing here
			b.Header.TimeUnix = &b.Header.Time.Seconds
		}
		res[k] = b
	}

	return c.JSON(res)
}

func (s *Server) StatsSummaryMiner(c *fiber.Ctx) error {
	var query statskMinerQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	if query.Days != 90 && query.Days != 365 {
		return c.Status(fiber.StatusBadRequest).SendString("Days query must either be 90 or 365")
	}

	period, err := time.ParseDuration("24h")
	if err != nil {
		return fmt.Errorf("invalid period duration: %w", err)
	}

	// map so that we have a unique list of all the fiel mask requested
	fieldMaskMap := make(map[string]struct{})

	current := time.Now().UTC()

	bod := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, current.Location())

	days := int(query.Days * -1)

	from := bod.AddDate(0, 0, days)
	to := current

	queryFields := statsSummaryQuery{
		Sum: []string{"stats.total_size", "stats.total_fee", "header.size"},
	}

	parsedProtoFieldTypes := parseComputeQueryToProtoFieldType(queryFields, fieldMaskMap)

	wocStatsRes, err := s.wocStatsClient.GetSummary(c.UserContext(), &woc_stats.GetSummaryRequest{
		From:    timestamppb.New(from),
		To:      timestamppb.New(to),
		Period:  durationpb.New(period),
		Fields:  parsedProtoFieldTypes,
		GroupBy: &fieldmaskpb.FieldMask{Paths: []string{"details.miner_tag"}},
	})
	if err != nil {
		return fmt.Errorf("failed to get summary from woc stats: %w", err)
	}

	fieldMask := make([]string, 0)

	// remove values before ".", as we assume it will always be "stats."
	for k := range fieldMaskMap {
		fieldMask = append(fieldMask, removePath(k))
	}

	// fieldmask_utils.MaskFromPaths check if the field mask is the same of the one request and unf it filter by the
	// field name being PascalCase, example: woc_stats.BlockStats.UtxoIncrease
	mask, err := fieldmask_utils.MaskFromPaths(fieldMask, strcase.ToCamel)
	if err != nil {
		return fmt.Errorf("failed to create a mask from paths: %w", err)
	}

	res := make([]map[string]interface{}, len(wocStatsRes.StatsPerPeriod))
	resStats := make(map[string]interface{})

	for k, statsPerPeriod := range wocStatsRes.StatsPerPeriod {
		for group, stats := range statsPerPeriod.Stats {
			// using local BlockStats, instead of proto, so that it has nil fields and return even for go zero-value fields
			var (
				blockStats  BlockStats
				blockHeader BlockStatsHeader
			)

			if err = fieldmask_utils.StructToStruct(mask, stats.BlockStats, &blockStats); err != nil {
				return fmt.Errorf("failed to create struct from field map: %w", err)
			}

			if err = fieldmask_utils.StructToStruct(mask, stats.BlockHeader, &blockHeader); err != nil {
				return fmt.Errorf("failed to create struct from field map: %w", err)
			}

			resStats[group] = map[string]interface{}{"stats": blockStats, "header": blockHeader, "count": stats.Count}
		}

		res[k] = map[string]interface{}{"period": statsPerPeriod.Period.Seconds, "stats": resStats}
		resStats = make(map[string]interface{}) // clean
	}

	return c.JSON(res)
}

func (s *Server) StatsTagCountByHeight(c *fiber.Ctx) error {
	elasticSearchEnabled := gocore.Config().GetBool("opReturnSearch", false)
	if !elasticSearchEnabled {
		// op return search not enabled. return
		logger.Log.Error("opreturn search disabled in config")
		return c.Status(fiber.StatusNotFound).SendString("Internal server error")
	}

	var height = c.Params("height")

	res, err := search.GetTagCountByBlockHeight(height)
	if err != nil {
		logger.Log.Error("failed to get tag count", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).SendString("Failed to  get tag count")
	}

	return c.JSON(res)
}

type MinerMinFeeRateQuery struct {
	From int64 `query:"from"`
	To   int64 `query:"to"`
}

type MinerMinFeeRate struct {
	Miner      string  `json:"miner"`
	MinFeeRate float64 `json:"min_fee_rate"`
}

func (s *Server) MinerMinFeeRates(c *fiber.Ctx) error {

	var query MinerMinFeeRateQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	if query.From == 0 || query.To == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("From and To query must be provided")
	}

	minerFeeRates, err := s.wocStatsClient.GetMinerMinFeeRates(c.UserContext(), &woc_stats.GetMinerMinFeeRatesRequest{
		From: timestamppb.New(time.Unix(query.From, 0)),
		To:   timestamppb.New(time.Unix(query.To, 0)),
	})
	if err != nil {
		return fmt.Errorf("failed to get Miner Min Fee Rates: %w", err)
	}

	minerFeeRatesList := make([]*MinerMinFeeRate, 0)

	for _, item := range minerFeeRates.MinerMinFeeRates {
		var miner string
		if item.Miner != "" {
			miner = item.Miner
		} else {
			miner = "Others"
		}

		minerFeeRatesList = append(minerFeeRatesList, &MinerMinFeeRate{
			Miner:      miner,
			MinFeeRate: item.MinFeeRate * 1000,
		})
	}

	return c.JSON(minerFeeRatesList)
}

type FeeRecommendation struct {
	// FeeUnit Unit for fee rate fields.
	FeeUnit string `json:"fee_unit"`
	// Fee Current fee for the unit.
	Fee int `json:"fee"`
	// MempoolMinFee Current mempool minimum relay fee (sat/vB).
	MempoolMinFee int `json:"mempool_min_fee"`

	FeeUSD string `json:"fee_usd,omitempty"`
}

func (s *Server) FeeRecommendation(c *fiber.Ctx) error {

	exchangeRate, err := price.GetUSDPrice()

	if err != nil {
		logger.Log.Error("FeeRecommendation Exchnage rate failure", zap.Error(err))
	}

	var feeUSD float64
	if exchangeRate > 0 {
		val := float64(configs.Settings.FeeRate) * 1e-8 * exchangeRate
		feeUSD = val
	}

	return c.JSON(FeeRecommendation{
		FeeUnit:       configs.Settings.FeeUnit,
		Fee:           configs.Settings.FeeRate,
		MempoolMinFee: configs.Settings.MinFee,
		FeeUSD:        fmt.Sprintf("%.8f", feeUSD),
	})
}
