package serverfiber

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/iancoleman/strcase"
	fieldmask_utils "github.com/mennanov/fieldmask-utils"
	commongrpc "github.com/teranode-group/common/grpc"
	"github.com/teranode-group/common/logger"
	woc_exchange_rate "github.com/teranode-group/proto/woc-exchange-rate"
	woc_stats "github.com/teranode-group/proto/woc-stats"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/redis"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type statsSummaryQuery struct {
	From    int
	To      int
	Period  string
	First   []string
	Last    []string
	Sum     []string
	Min     []string
	Max     []string
	Avg     []string
	GroupBy []string `query:"group_by"`
}

func (s *Server) StatsSummary(c *fiber.Ctx) error {
	var query statsSummaryQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	period, err := time.ParseDuration(query.Period)
	if err != nil {
		return fmt.Errorf("invalid period duration: %w", err)
	}

	// map so that we have a unique list of all the fiel mask requested
	fieldMaskMap := make(map[string]struct{})

	from := time.Unix(int64(query.From), 0)
	to := time.Unix(int64(query.To), 0)

	parsedProtoFieldTypes := parseComputeQueryToProtoFieldType(query, fieldMaskMap)

	for _, f := range parsedProtoFieldTypes {
		fmt.Printf("Sending field: %v | Paths: %v | Compute: %v\n", f, f.FieldMask.Paths, f.Compute)
	}

	wocStatsRes, err := s.wocStatsClient.GetSummary(c.UserContext(), &woc_stats.GetSummaryRequest{
		From:    timestamppb.New(from),
		To:      timestamppb.New(to),
		Period:  durationpb.New(period),
		Fields:  parsedProtoFieldTypes,
		GroupBy: &fieldmaskpb.FieldMask{Paths: query.GroupBy},
	})
	if err != nil {
		return fmt.Errorf("failed to get summary from woc stats: %w", err)
	}

	allPaths := []string{}
	for _, ft := range parsedProtoFieldTypes {
		allPaths = append(allPaths, ft.FieldMask.GetPaths()...)
	}
	combinedMask := &fieldmaskpb.FieldMask{Paths: allPaths}

	exchangeRates, err := s.fetchStatsExchangeRates(combinedMask, from, to, c.Context())

	if err != nil {
		logger.Log.Error("Exchange request failure", zap.Error(err))
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

			setBlockStatsUsdFees(exchangeRates, statsPerPeriod.Period.AsTime(), &blockStats, stats.BlockStats)
			setTxRatePerSecond(&blockStats, stats.BlockHeader, query.Period)

			resStats[group] = map[string]interface{}{"stats": blockStats, "header": blockHeader, "count": stats.Count}
		}

		res[k] = map[string]interface{}{"period": statsPerPeriod.Period.Seconds, "stats": resStats}
		resStats = make(map[string]interface{}) // clean
	}

	return c.JSON(res)
}

type statsBlockQuery struct {
	From        int64    `query:"from"`
	To          int64    `query:"to"`
	Fields      []string `query:"fields"`
	OrderByAsc  []string `query:"order_by_asc"`
	OrderByDesc []string `query:"order_by_desc"`
}

func (s *Server) StatsBlock(c *fiber.Ctx) error {

	var query statsBlockQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	query.Fields = SplitCommaSeparatedValues(query.Fields)

	res, err := s.GetBlockStats(c.UserContext(), query)
	if err != nil {
		return fmt.Errorf("failed to get block stats: %w", err)
	}

	return c.JSON(res)
}

func (s *Server) GetBlockStats(ctx context.Context, query statsBlockQuery) ([]Block, error) {

	fieldMask, err := fieldmaskpb.New(&woc_stats.Block{}, query.Fields...)
	if err != nil {
		return nil, fmt.Errorf("invalid fields: %w", err)
	}

	orderBy := make([]*woc_stats.OrderBy, 0)

	if len(query.OrderByAsc) > 0 {
		fieldMask, err := fieldmaskpb.New(&woc_stats.Block{}, query.OrderByAsc...)
		if err != nil {
			return nil, fmt.Errorf("invalid order by asc: %w", err)
		}

		orderBy = append(orderBy, &woc_stats.OrderBy{
			FieldMask: fieldMask,
			Order:     woc_stats.Order_ASC,
		})
	}

	if len(query.OrderByDesc) > 0 {
		fieldMask, err := fieldmaskpb.New(&woc_stats.Block{}, query.OrderByDesc...)
		if err != nil {
			return nil, fmt.Errorf("invalid order by desc: %w", err)
		}

		orderBy = append(orderBy, &woc_stats.OrderBy{
			FieldMask: fieldMask,
			Order:     woc_stats.Order_DESC,
		})
	}

	from := time.Unix(query.From, 0)
	to := time.Unix(query.To, 0)

	wocStatsRes, err := s.wocStatsClient.GetBlock(ctx, &woc_stats.GetBlockRequest{
		From:      timestamppb.New(from),
		To:        timestamppb.New(to),
		FieldMask: fieldMask,
		OrderBy:   orderBy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get block from woc stats: %w", err)
	}

	mask, err := fieldmask_utils.MaskFromPaths(query.Fields, strcase.ToCamel)
	if err != nil {
		return nil, fmt.Errorf("failed to create a mask from paths: %w", err)
	}

	res := make([]Block, len(wocStatsRes.Blocks))

	exchangeRates, err := s.fetchStatsExchangeRates(fieldMask, from, to, ctx)

	if err != nil {
		logger.Log.Error("Exchange rate failure", zap.Error(err))
	}

	for k, block := range wocStatsRes.Blocks {
		var b Block

		// using local BlockStats so that it has nil fields and return when on zero-value
		if err = fieldmask_utils.StructToStruct(mask, block, &b); err != nil {
			return nil, fmt.Errorf("failed to create struct from field map: %w", err)
		}

		if b.Header.Time != nil { // nolint: staticcheck // not sure why lint failing here
			b.Header.TimeUnix = &b.Header.Time.Seconds
		}

		setBlockStatsUsdFees(exchangeRates, b.Header.Time.AsTime(), b.Stats, block.Stats)

		res[k] = b
	}

	return res, nil
}

func parseComputeQueryToProtoFieldType(query statsSummaryQuery, fieldMaskMap map[string]struct{}) []*woc_stats.FieldType {
	fields := make([]*woc_stats.FieldType, 0)

	type computeEntry struct {
		rawPaths []string
		compute  woc_stats.Compute
	}

	entries := []computeEntry{
		{rawPaths: query.Avg, compute: woc_stats.Compute_AVG},
		{rawPaths: query.Sum, compute: woc_stats.Compute_SUM},
		{rawPaths: query.Last, compute: woc_stats.Compute_LAST},
		{rawPaths: query.Max, compute: woc_stats.Compute_MAX},
		{rawPaths: query.Min, compute: woc_stats.Compute_MIN},
		{rawPaths: query.First, compute: woc_stats.Compute_FIRST},
	}

	for _, entry := range entries {
		paths := SplitCommaSeparatedValues(entry.rawPaths)
		for _, path := range paths {
			fields = append(fields, &woc_stats.FieldType{
				FieldMask: &fieldmaskpb.FieldMask{Paths: []string{path}},
				Compute:   entry.compute,
			})
			fieldMaskMap[path] = struct{}{}
		}
	}

	return fields
}

func removePath(in string) string {
	index := strings.SplitAfter(in, ".")
	if len(index) > 1 {
		return index[1]
	}

	return in
}

type BlockStats struct {
	AvgFee                     *int64      `json:"avg_fee,omitempty"`
	AvgFeeUsd                  json.Number `json:"avg_fee_usd,omitempty"`
	AvgFeeRate                 *int64      `json:"avg_fee_rate,omitempty"`
	AvgFeeRateSubsats          *float64    `json:"avg_fee_rate_subsats,omitempty"`
	AvgTxSize                  *int64      `json:"avg_tx_size,omitempty"`
	Ins                        *int64      `json:"ins,omitempty"`
	MaxFee                     *int64      `json:"max_fee,omitempty"`
	MaxFeeRate                 *int64      `json:"max_fee_rate,omitempty"`
	MaxFeeRateSubsats          *float64    `json:"max_fee_rate_subsats,omitempty"`
	MaxTxSize                  *int64      `json:"max_tx_size,omitempty"`
	MedianFee                  *int64      `json:"median_fee,omitempty"`
	MedianFeeUsd               json.Number `json:"median_fee_usd,omitempty"`
	MedianFeeRate              *int64      `json:"median_fee_rate,omitempty"`
	MedianTxSize               *int64      `json:"median_tx_size,omitempty"`
	MinFee                     *int64      `json:"min_fee,omitempty"`
	MinFeeRate                 *int64      `json:"min_fee_rate,omitempty"`
	MinFeeRateSubsats          *float64    `json:"min_fee_rate_subsats,omitempty"`
	MinTxSize                  *int64      `json:"min_tx_size,omitempty"`
	Outs                       *int64      `json:"outs,omitempty"`
	Subsidy                    *int64      `json:"subsidy,omitempty"`
	TotalOut                   *int64      `json:"total_out,omitempty"`
	TotalSize                  *int64      `json:"total_size,omitempty"`
	TotalFee                   *int64      `json:"total_fee,omitempty"`
	TotalFeeUsd                json.Number `json:"total_fee_usd,omitempty"`
	MinersRevenueUsd           json.Number `json:"miners_revenue_usd,omitempty"`
	UtxoIncrease               *int64      `json:"utxo_increase,omitempty"`
	CddTotal                   *float64    `json:"cdd_total,omitempty"`
	HashRate                   *float64    `json:"hash_rate,omitempty"`
	TotalUtxoIncrease          *int64      `json:"total_utxo_increase,omitempty"`
	TotalBlockSize             *int64      `json:"total_block_size,omitempty"`
	TxRatePerSecond            *float64    `json:"tx_rate_per_second,omitempty"`
	TotalTxCount               *int64      `json:"total_tx_count,omitempty"`
	CirculatingSupply          *int64      `json:"circulating_supply,omitempty"`
	TotalOutsAboveZero         *int64      `json:"total_outs_above_zero,omitempty"`
	TotalUtxoIncreaseAboveZero *int64      `json:"total_utxo_increase_above_zero,omitempty"`
}

type BlockStatsHeader struct {
	Hash       *string                `json:"hash,omitempty"`
	TxCount    *int64                 `json:"tx_count,omitempty"`
	Size       *int64                 `json:"size,omitempty"`
	Height     *uint64                `json:"height,omitempty"`
	Version    *uint64                `json:"version,omitempty"`
	VersionHex *string                `json:"version_hex,omitempty"`
	MerkleRoot *string                `json:"merkle_root,omitempty"`
	Time       *timestamppb.Timestamp `json:"-"`
	TimeUnix   *int64                 `json:"time,omitempty"`
	MedianTime *uint64                `json:"median_time,omitempty"`
	Nonce      *uint64                `json:"nonce,omitempty"`
	Bits       *string                `json:"bits,omitempty"`
	Difficulty *float64               `json:"difficulty,omitempty"`
	Chainwork  *string                `json:"chainwork,omitempty"`
	BlockSize  *uint64                `json:"block_size,omitempty"`
}

type BlockDetails struct {
	Coinbase *string `json:"coinbase,omitempty"`
	MinerTag *string `json:"miner_tag,omitempty"`
}

type Block struct {
	Header  *BlockStatsHeader `protobuf:"bytes,1,opt,name=header,proto3" json:"header,omitempty"`
	Stats   *BlockStats       `protobuf:"bytes,2,opt,name=stats,proto3" json:"stats,omitempty"`
	Details *BlockDetails     `protobuf:"bytes,3,opt,name=details,proto3" json:"details,omitempty"`
}

func WocStatsHealthCheck(c context.Context) (*woc_stats.HealthCheckResponse, error) {
	wocStatsConnection, err := commongrpc.NewClientConnection(configs.Settings.WocStatsAddress, logger.Log, nil)
	if err != nil {
		logger.Log.Info("Failed to connect to woc-stats service", zap.Error(err))
		return &woc_stats.HealthCheckResponse{}, fmt.Errorf("failed to connect to woc-stats service: %w", err)
	}

	defer wocStatsConnection.Close()

	healthCheckClient := woc_stats.NewHealthClient(wocStatsConnection)

	wocStatsHealthResponse, err := healthCheckClient.Check(
		c, &woc_stats.HealthCheckRequest{
			Service: "woc-stats",
		})

	if err != nil {
		logger.Log.Info("Failed to get woc-stats health check", zap.Error(err))
		return &woc_stats.HealthCheckResponse{}, fmt.Errorf("failed to get woc-stats health check: %w", err)
	}

	return wocStatsHealthResponse, nil
}

type blocksQuery struct {
	Offset  int64  `query:"offset"`
	Limit   int64  `query:"limit"`
	OrderBy string `query:"order_by"`
	SortBy  string `query:"sort_by"`
	Filter  string `query:"filter"`
}

type BlocksResponse struct {
	Blocks     []BlockHeader `json:"blocks"`
	TotalCount uint64        `json:"totalCount"`
}

func (s *Server) Blocks(c *fiber.Ctx) error {

	var query blocksQuery
	var orderBy woc_stats.Order
	var sortBy woc_stats.BLOCK_FIELDS
	var filterBy woc_stats.BLOCK_FIELDS
	var filterValue string

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	if len(query.OrderBy) > 0 {
		value, ok := woc_stats.Order_value[strings.ToUpper(query.OrderBy)]
		if !ok {
			return fmt.Errorf("invalid order_by field")
		}
		orderBy = woc_stats.Order(value)
	} else {
		orderBy = woc_stats.Order_DESC
	}

	if len(query.SortBy) > 0 {
		value, ok := woc_stats.BLOCK_FIELDS_value[strings.ToUpper(query.SortBy)]
		if !ok {
			return fmt.Errorf("invalid sort_by field")
		}
		sortBy = woc_stats.BLOCK_FIELDS(value)
	} else {
		sortBy = woc_stats.BLOCK_FIELDS_HEIGHT
	}

	if len(query.Filter) > 0 {
		var filterStrings = strings.Split(query.Filter, ":")
		if len(filterStrings) < 2 {
			return fmt.Errorf("invalid filter query provided")
		}
		value, ok := woc_stats.BLOCK_FIELDS_value[strings.ToUpper(filterStrings[0])]
		if !ok {
			return fmt.Errorf("invalid filter_by field")
		}
		filterBy = woc_stats.BLOCK_FIELDS(value)
		filterValue = filterStrings[1]
	} else {
		filterBy = woc_stats.BLOCK_FIELDS_UNKNOWN
		filterValue = ""
	}

	if query.Limit == 0 {
		query.Limit = 10
	}

	wocBlocksRes, err := s.wocStatsClient.GetBlocks(c.UserContext(), &woc_stats.GetBlocksRequest{
		Limit:       query.Limit,
		Offset:      query.Offset,
		OrderBy:     orderBy,
		SortBy:      sortBy,
		FilterBy:    filterBy,
		FilterValue: filterValue,
	})
	if err != nil {
		return fmt.Errorf("failed to get blocks from woc stats: %w", err)
	}

	res := make([]BlockHeader, len(wocBlocksRes.Blocks))

	for k, block := range wocBlocksRes.Blocks {
		var b BlockHeader

		b.Hash = &block.Hash
		b.TxCount = &block.TxCount
		b.BlockSize = &block.BlockSize
		b.Version = &block.Version
		b.VersionHex = &block.VersionHex
		b.MerkleRoot = &block.MerkleRoot
		b.Height = &block.Height
		b.Time = block.Time
		b.MedianTime = &block.MedianTime
		b.Nonce = &block.Nonce
		b.Bits = &block.Bits
		b.Difficulty = &block.Difficulty
		b.Chainwork = &block.Chainwork
		b.MinerTag = &block.MinerTag
		b.TotalFee = &block.TotalFee
		b.AvgFee = &block.AvgFee
		if b.Time != nil {
			b.TimeUnix = &b.Time.Seconds
		}
		b.Coinbase = &block.Coinbase

		if block.MinerTag == "" {
			if block.MinerTag == "" {
				src := []byte(block.Coinbase)
				dst := make([]byte, hex.DecodedLen(len(src)))
				n, err := hex.Decode(dst, src)
				if err != nil {
					b.MinerTag = &block.Coinbase
				} else {
					tag := fmt.Sprintf("%s\n", dst[:n])
					b.MinerTag = &tag
				}
			}
		}

		b.Coinbase = &block.Coinbase
		res[k] = b
	}

	return c.JSON(&BlocksResponse{Blocks: res, TotalCount: wocBlocksRes.TotalCount})

}

func (s *Server) MinerTags(c *fiber.Ctx) error {

	minerTagsRes, err := s.wocStatsClient.GetMinerTags(c.UserContext(), &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("failed to get Miner tags: %w", err)
	}

	return c.JSON(minerTagsRes.MinerTags)
}

type HomePage24HourStats struct {
	TxCount24HourStats      []Block                  `json:"tx_count_24_hour_stats"`
	MinerTags24HourStats    []Block                  `json:"miner_tags_24_hour_stats"`
	TagsSummary24hourStats  []*woc_stats.PeriodStats `json:"tags_summary_24_hour_stats"`
	ExchangeRate24HourStats []ExchangeRate           `json:"exchnageRate_24_hour_stats"`
	LastUpdate              int64                    `json:"last_update"`
}

func (s *Server) HomePageStatsFor24Hours(c *fiber.Ctx) error {

	resp, err := s.getCachedHomePageStatsFor24Hours()
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

func (s *Server) TagsSummaryByHeight(c *fiber.Ctx) error {
	heightParam := c.Params("height")
	if heightParam == "" {
		return fmt.Errorf("height parameter is required")
	}

	height, err := strconv.ParseInt(heightParam, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid height parameter: %w", err)
	}

	tagsSummaryRes, err := s.wocStatsClient.GetTagsSummaryByHeight(c.UserContext(),
		&woc_stats.GetTagsSummaryByHeightRequest{
			Height: height,
		})

	if err != nil {
		return fmt.Errorf("failed to get tags summary by height: %w", err)
	}

	if len(tagsSummaryRes.TagStatsByBlock) == 0 {
		return c.JSON([]*woc_stats.StatsByTag{})
	}

	return c.JSON(tagsSummaryRes.TagStatsByBlock)
}

func (s *Server) TagsSummary(c *fiber.Ctx) error {
	var days int
	var err error
	var resp []*woc_stats.PeriodStats

	daysParam := c.Params("days")
	if daysParam == "" {
		return fmt.Errorf("days parameter is required")
	}

	if daysParam == "all" {
		days = MAX_TAGS_DAYS
	} else {
		days, err = strconv.Atoi(daysParam)
	}

	if err != nil {
		return fmt.Errorf("invalid days paramater")
	}

	if days != 1 && days != 7 && days != 30 && days != 90 && days != 365 && days != MAX_TAGS_DAYS {
		return fmt.Errorf("invalid number of days: %d", days)
	}

	resp, err = s.getCachedTagsSummaryData(c.Context(), days)

	if err != nil {
		return err
	}

	return c.JSON(resp)
}

func (s *Server) GetTagsSummary(ctx context.Context, days int) ([]*woc_stats.PeriodStats, error) {

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -days)

	var periodString string
	if days == 1 {
		periodString = "1h"
	} else if days == MAX_TAGS_DAYS || days == 365 {
		periodString = "168h"
	} else {
		periodString = "24h"
	}

	period, err := time.ParseDuration(periodString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse duration: %v", err)
	}

	tagsSummaryRes, err := s.wocStatsClient.GetTagsSummary(ctx,
		&woc_stats.GetTagsSummaryRequest{
			From:   timestamppb.New(from),
			To:     timestamppb.New(now),
			Period: durationpb.New(period),
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get tags summary: %w", err)
	}

	return tagsSummaryRes.StatsPerPeriod, nil
}

func (s *Server) getCachedTagsSummaryData(ctx context.Context, days int) ([]*woc_stats.PeriodStats, error) {
	var cached []*woc_stats.PeriodStats
	var err error

	if redis.RedisClient.Enabled {
		if key, exists := TagsSummaryCacheKeys[days]; exists {
			err = redis.GetCachedValue(key, &cached, nil)
			if err == nil && cached != nil {
				// Cache hit
				return cached, nil
			}
		}
	}

	cached, err = s.GetTagsSummary(ctx, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags summary: %w", err)
	}

	return cached, nil
}

func (s *Server) getCachedHomePageStatsFor24Hours() (HomePage24HourStats, error) {
	var stats HomePage24HourStats
	var txCount24HourStatsCache []Block
	var minerTags24HourStatsCache []Block
	var tagsSummary24hourStatsCache []*woc_stats.PeriodStats
	var exchangeRate24HourStatCache []ExchangeRate
	var lastUpdate int64
	var err error

	if redis.RedisClient.Enabled {
		err = redis.GetCachedValue(KEY_TxCountForLast24Hours, &txCount24HourStatsCache, nil)
		if err == nil && txCount24HourStatsCache != nil {
			stats.TxCount24HourStats = txCount24HourStatsCache
		} else {
			return HomePage24HourStats{}, fmt.Errorf("failed to get HomePage stats: %w", err)
		}

		err = redis.GetCachedValue(KEY_MinerTagStatsForLast24Hours, &minerTags24HourStatsCache, nil)
		if err == nil && minerTags24HourStatsCache != nil {
			stats.MinerTags24HourStats = minerTags24HourStatsCache
		} else {
			return HomePage24HourStats{}, fmt.Errorf("failed to get HomePage stats: %w", err)
		}

		err = redis.GetCachedValue(KEY_GetTagsSummaryFromDay, &tagsSummary24hourStatsCache, nil)
		if err == nil && tagsSummary24hourStatsCache != nil {
			stats.TagsSummary24hourStats = tagsSummary24hourStatsCache
		} else {
			return HomePage24HourStats{}, fmt.Errorf("failed to get HomePage stats: %w", err)
		}

		err = redis.GetCachedValue(KEY_ExchangeRateForLast24Hours, &exchangeRate24HourStatCache, nil)
		if err == nil && exchangeRate24HourStatCache != nil {
			stats.ExchangeRate24HourStats = exchangeRate24HourStatCache
		} else {
			return HomePage24HourStats{}, fmt.Errorf("failed to get HomePage stats: %w", err)
		}

		err = redis.GetCachedValue(KEY_LastHomePageStatsUpdateFor24Hours, &lastUpdate, nil)
		if err == nil && lastUpdate != 0 {
			stats.LastUpdate = lastUpdate
		} else {
			return HomePage24HourStats{}, fmt.Errorf("failed to get HomePage stats: %w", err)
		}

		return stats, nil
	}
	return HomePage24HourStats{}, fmt.Errorf("failed to get HomePage stats: %w", err)
}

func (s *Server) fetchStatsExchangeRates(fieldMask *fieldmaskpb.FieldMask, from time.Time, to time.Time, c context.Context) ([]*woc_exchange_rate.ExchangeRate, error) {

	var rates = make([]*woc_exchange_rate.ExchangeRate, 0)

	var validFields int16

	for _, path := range fieldMask.Paths {
		if path == "stats.avg_fee" ||
			path == "stats.median_fee" ||
			path == "stats.total_fee" {
			validFields = validFields + 1
		}

	}

	if validFields == 0 {
		return rates, nil
	}

	wocExchangeRateRes, err := s.wocExchangeRateClient.GetHistoricalExchangeRate(c, &woc_exchange_rate.GetExchangeRateRequest{
		From: timestamppb.New(from),
		To:   timestamppb.New(to),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rates: %w", err)
	}

	rates = wocExchangeRateRes.ExchangeRates

	var latestExchangeRateRes *woc_exchange_rate.GetLatestExchangeRateResponse

	wocExchangeRateReq := &woc_exchange_rate.GetLatestExchangeRateRequest{
		Coin: woc_exchange_rate.CoinType_BSV,
	}
	latestExchangeRateRes, err = s.wocExchangeRateClient.GetLatestExchangeRate(c, wocExchangeRateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest exchange rate: %w", err)
	}
	rates = append(rates, latestExchangeRateRes.ExchangeRate)

	return rates, nil
}

func setBlockStatsUsdFees(exchangeRates []*woc_exchange_rate.ExchangeRate, time time.Time, stats *BlockStats, blockStats *woc_stats.BlockStats) {

	if len(exchangeRates) > 0 {
		exchangeRate := getStatsBlockExchangeRate(exchangeRates, time)

		if exchangeRate.Rate != 0 {

			if blockStats.MedianFee != 0 {
				medianFeeUsd := float64(blockStats.MedianFee) * (exchangeRate.Rate / 1e8)
				stats.MedianFeeUsd = json.Number(strconv.FormatFloat(medianFeeUsd, 'f', -1, 64))
			}
			if blockStats.AvgFee != 0 {
				avgFeeUsd := float64(blockStats.AvgFee) * (exchangeRate.Rate / 1e8)
				stats.AvgFeeUsd = json.Number(strconv.FormatFloat(avgFeeUsd, 'f', -1, 64))
			}
			if blockStats.TotalFee != 0 {
				totalFeeUsd := float64(blockStats.TotalFee) * (exchangeRate.Rate / 1e8)
				stats.TotalFeeUsd = json.Number(strconv.FormatFloat(totalFeeUsd, 'f', -1, 64))
			}

			if blockStats.TotalFee != 0 && blockStats.Subsidy != 0 {
				minersRevenue := blockStats.TotalFee + blockStats.Subsidy
				minersRevenueUsd := float64(minersRevenue) * (exchangeRate.Rate / 1e8)
				stats.MinersRevenueUsd = json.Number(strconv.FormatFloat(minersRevenueUsd, 'f', -1, 64))
			}
		}
	}
}

func setTxRatePerSecond(stats *BlockStats, blockHeader *woc_stats.BlockStatsHeader, periodQuery string) {
	if blockHeader.TxCount != 0 && periodQuery == "24h" {
		// Calculate rates per second over the last 24 hours
		txRatePerSecond := float64(blockHeader.TxCount) / float64(86400) // Number of seconds in 24 hour
		stats.TxRatePerSecond = &txRatePerSecond
	}
}

func getStatsBlockExchangeRate(exchnageRates []*woc_exchange_rate.ExchangeRate, period time.Time) *woc_exchange_rate.ExchangeRate {
	for _, rate := range exchnageRates {
		if rate.Time.AsTime().Truncate(24 * time.Hour).Equal(period.Truncate(24 * time.Hour)) {
			return rate
		}
	}
	return &woc_exchange_rate.ExchangeRate{}
}

type DailyStats struct {
	Time                        *int64   `json:"time,omitempty"`
	TotalUtxoIncrease           *int64   `json:"total_utxo_increase,omitempty"`
	TotalBlockSize              *int64   `json:"total_block_size,omitempty"`
	TotalHashRate               *string  `json:"total_hash_rate,omitempty"`
	MedianBlockConfirmationTime *float64 `json:"median_block_confirmation_time,omitempty"`
	TotalTxCount                *int64   `json:"total_tx_count,omitempty"`
	TxRatePerSecond             *float64 `json:"tx_rate_per_second,omitempty"`
	CirculatingSupply           *int64   `json:"circulating_supply,omitempty"`
	TotalOutsAboveZero          *int64   `json:"total_outs_above_zero,omitempty"`
	TotalUtxoIncreaseAboveZero  *int64   `json:"total_utxo_increase_above_zero,omitempty"`
}

type DailyStatsQuery struct {
	From int64
	To   int64
}

func (s *Server) DailyStatsSummary(c *fiber.Ctx) error {
	var query DailyStatsQuery

	if err := c.QueryParser(&query); err != nil {
		return fmt.Errorf("failed to read fields: %w", err)
	}

	dailyStatsSummaryRes, err := s.wocStatsClient.GetDailyStatsSummary(c.UserContext(), &woc_stats.GetDailyStatsSummaryRequest{
		From: timestamppb.New(time.Unix(query.From, 0)),
		To:   timestamppb.New(time.Unix(query.To, 0)),
	})
	if err != nil {
		return fmt.Errorf("failed to get GetDailyStatsSummary: %w", err)
	}
	res := make([]DailyStats, len(dailyStatsSummaryRes.DailyStats))

	for k, dailyStat := range dailyStatsSummaryRes.DailyStats {
		res[k] = DailyStats{
			Time:                        &dailyStat.Time.Seconds,
			TotalUtxoIncrease:           &dailyStat.TotalUtxoIncrease,
			TotalBlockSize:              &dailyStat.TotalBlockSize,
			TotalHashRate:               &dailyStat.TotalHashRate,
			MedianBlockConfirmationTime: &dailyStat.MedianBlockConfirmationTime,
			TotalTxCount:                &dailyStat.TotalTxCount,
			CirculatingSupply:           &dailyStat.CirculatingSupply,
			TotalOutsAboveZero:          &dailyStat.TotalOutsAboveZero,
			TotalUtxoIncreaseAboveZero:  &dailyStat.TotalUtxoIncreaseAboveZero,
		}
	}

	return c.JSON(res)
}

func (s *Server) StatsQuery(c *fiber.Ctx) error {
	// Parse JSON body for the query
	var in struct {
		Query string `json:"query"`
	}
	if err := c.BodyParser(&in); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if strings.TrimSpace(in.Query) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "field 'query' is required"})
	}

	// Pick mode from query param (?mode=summary|markdown)
	mode := strings.ToLower(c.Query("mode", "summary"))
	var endpoint string
	switch mode {
	case "markdown":
		endpoint = "/nl-table"
	default:
		endpoint = "/nl-explain"
	}

	upstream := strings.TrimRight(configs.Settings.WocStatsMcpClientUrl, "/") + endpoint

	// Build payload { q: "..." }
	payload := map[string]any{"q": in.Query}
	body, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(c.Context(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream, bytes.NewReader(body))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{"error": "upstream timeout"})
		}
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "failed to read upstream response"})
	}

	if mode == "markdown" {
		var nodeResp struct {
			Markdown string `json:"markdown"`
		}
		if err := json.Unmarshal(respBody, &nodeResp); err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "invalid upstream JSON"})
		}
		return c.JSON(fiber.Map{"markdown": nodeResp.Markdown})
	}

	// default: summary
	var nodeResp struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(respBody, &nodeResp); err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "invalid upstream JSON"})
	}
	return c.JSON(fiber.Map{"summary": nodeResp.Summary})
}

type taggedOutputsQuery struct {
	FullTag string `query:"full_tag"`
	Limit   uint32 `query:"limit"`
	Offset  uint32 `query:"offset"`
	OrderBy string `query:"order_by"`
}

type TaggedOutputRow struct {
	Created string `json:"created"`
	FullTag string `json:"fulltag"`
	Height  int64  `json:"height"`
	Tag     string `json:"tag"`
	TxID    string `json:"txid"`
	Vout    int32  `json:"vout"`
}

type TaggedOutputsResponse struct {
	Outputs    []TaggedOutputRow `json:"outputs"`
	TotalCount uint64            `json:"total_count"`
}

func (s *Server) TaggedOutputs(c *fiber.Ctx) error {
	var q taggedOutputsQuery
	if err := c.QueryParser(&q); err != nil {
		return fmt.Errorf("failed to parse query: %w", err)
	}
	if strings.TrimSpace(q.FullTag) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "full_tag is required")
	}

	// defaults
	if q.Limit == 0 {
		q.Limit = 10
	}
	orderBy := woc_stats.Order_DESC
	if q.OrderBy != "" {
		if v, ok := woc_stats.Order_value[strings.ToUpper(q.OrderBy)]; ok {
			orderBy = woc_stats.Order(v)
		} else {
			return fiber.NewError(fiber.StatusBadRequest, "invalid order_by (use asc|desc)")
		}
	}

	res, err := s.wocStatsClient.GetTaggedOutputs(c.UserContext(), &woc_stats.GetTaggedOutputsRequest{
		FullTag: q.FullTag,
		Limit:   q.Limit,
		Offset:  q.Offset,
		OrderBy: orderBy,
	})
	if err != nil {
		return fmt.Errorf("woc-stats GetTaggedOutputs failed: %w", err)
	}

	// map to JSON
	rows := make([]TaggedOutputRow, 0, len(res.GetOutputs()))
	for _, o := range res.GetOutputs() {
		created := ""
		if ts := o.GetCreatedAt(); ts != nil {
			created = ts.AsTime().UTC().Format(time.RFC3339)
		}
		rows = append(rows, TaggedOutputRow{
			Created: created,
			FullTag: o.GetFullTag(),
			Height:  o.GetHeight(),
			Tag:     o.GetTag(),
			TxID:    o.GetTxid(),
			Vout:    o.GetVout(),
		})
	}

	return c.JSON(&TaggedOutputsResponse{
		Outputs:    rows,
		TotalCount: res.GetTotalCount(),
	})
}

func (s *Server) SearchFullTag(c *fiber.Ctx) error {
	// 1️⃣ Parse query params
	query := strings.TrimSpace(c.Query("query"))
	if query == "" {
		return fiber.NewError(fiber.StatusBadRequest, "query is required")
	}

	// 2️⃣ Call woc-stats gRPC SearchFullTag
	res, err := s.wocStatsClient.SearchFullTag(c.UserContext(), &woc_stats.GetTaggedOutputsRequest{
		FullTag: query,
	})
	if err != nil {
		return fmt.Errorf("woc-stats SearchFullTag failed: %w", err)
	}

	// 3️⃣ Return simple JSON
	return c.JSON(fiber.Map{
		"full_tag": res.GetFullTag(),
	})
}

func SplitCommaSeparatedValues(input []string) []string {
	var result []string
	for _, v := range input {
		parts := strings.Split(v, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}
