package serverfiber

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/teranode-group/woc-api/configs"
)

func (s *Server) createRoutes() {
	ui := s.app.Group("/ui")
	uiStats := ui.Group("/stats")
	exchangeRate := s.app.Group("/exchangerate")
	block := s.app.Group("/block")
	tx := s.app.Group("/tx")
	miner := s.app.Group("/miner")
	utxo := s.app.Group("/utxos")
	utxoByAddress := s.app.Group("/address")
	uiUtxoByAddress := ui.Group("/address")
	bulkByAddress := s.app.Group("/addresses")
	bulkByScriptHash := s.app.Group("/scripts")
	utxoByScriptHash := s.app.Group("/script")
	uiUtxoByScriptHash := ui.Group("/script")

	uiStats.Use(func(c *fiber.Ctx) error {
		if !configs.Settings.WocStatsEnabled {
			return fmt.Errorf("not enabled")
		}
		return c.Next()
	})

	// block.Use(func(c *fiber.Ctx) error {
	// 	if !configs.Settings.WocStatsEnabled {
	// 		return fmt.Errorf("not enabled")
	// 	}
	// 	return c.Next()
	// })

	s.app.Get("/feerecommendation", s.FeeRecommendation)

	ui.Get("/tx/propagation/:txid", s.TxPropagation)
	ui.Get("/homepage/stats", s.HomePageStatsFor24Hours)
	ui.Get("/taggedoutputs", s.TaggedOutputs)
	ui.Get("/searchtagoutput", s.SearchFullTag)
	uiStats.Get("/chart/summary", s.StatsSummary)
	uiStats.Get("/query", s.StatsSummary)
	uiStats.Get("/chart/blocks", s.StatsBlock)
	uiStats.Get("/chart/summary", s.StatsSummary)
	uiStats.Get("/chart/summary", s.StatsSummary)
	uiStats.Get("/chart/dailysummary", s.DailyStatsSummary)
	uiStats.Get("/mempool", s.GetMempoolStats)
	miner.Get("/blocks/stats", s.StatsBlockMiner)
	miner.Get("/fees", s.MinerMinFeeRates)
	miner.Get("/summary/stats", s.StatsSummaryMiner)
	uiStats.Get("/blocks", s.Blocks)
	uiStats.Get("/minertags", s.MinerTags)
	uiStats.Get("/tagssummary/:days", s.TagsSummary)
	uiStats.Get("/tagssummary/height/:height", s.TagsSummaryByHeight)

	block.Get("/:from-:to/stats", s.BlocksByHeightRange)
	block.Get("/height/:height/stats", s.Block)
	block.Get("/hash/:hash/stats", s.Block)
	block.Get("/height/:height/txindex/:txindex", s.RawTransactionByBlockHeightAndTxIndex)
	block.Get("/tagcount/height/:height/stats", s.StatsTagCountByHeight)
	block.Get("/headers/resources", s.BlockHeadersFileResources)
	block.Get("/headers/latest", s.BlockHeadersLatest)
	block.Get("/headers/:filename", s.BlockHeadersFile)
	exchangeRate.Get("/", s.LatestExchangeRate)
	exchangeRate.Get("/historical", s.HistoricalExchangeRate)
	exchangeRate.Get("/latest", s.ExchangeRate)
	utxoByAddress.Get("/:addressOrScripthash/confirmed/history", s.GetAddressConfirmedHistory)
	utxoByAddress.Get("/:addressOrScripthash/confirmed/unspent", s.GetAddressConfirmedUnspent)
	utxoByAddress.Get("/:addressOrScripthash/confirmed/balance", s.GetAddressConfirmedBalance)
	utxoByAddress.Get("/:addressOrScripthash/unconfirmed/history", s.GetAddressMempoolHistory)
	utxoByAddress.Get("/:addressOrScripthash/unconfirmed/unspent", s.GetAddressMempoolUnspent)
	utxoByAddress.Get("/:addressOrScripthash/unconfirmed/balance", s.GetAddressMempoolBalance)
	utxoByAddress.Get("/:address/used", s.IsAdressInUse)
	utxoByAddress.Get("/:address/scripts", s.GetAddressScripts)

	utxoByAddress.Get("/:addressOrScripthash/unspent/all", s.GetAddressUnspentAll)

	uiUtxoByAddress.Get("/:addressOrScripthash/stats", s.GetAddressStats)

	utxoByScriptHash.Get("/:addressOrScripthash/confirmed/history", s.GetAddressConfirmedHistory)
	utxoByScriptHash.Get("/:addressOrScripthash/confirmed/unspent", s.GetAddressConfirmedUnspent)
	utxoByScriptHash.Get("/:addressOrScripthash/confirmed/balance", s.GetAddressConfirmedBalance)
	utxoByScriptHash.Get("/:addressOrScripthash/unconfirmed/history", s.GetAddressMempoolHistory)
	utxoByScriptHash.Get("/:addressOrScripthash/unconfirmed/unspent", s.GetAddressMempoolUnspent)
	utxoByScriptHash.Get("/:addressOrScripthash/unconfirmed/balance", s.GetAddressMempoolBalance)

	utxoByScriptHash.Get("/:addressOrScripthash/unspent/all", s.GetAddressUnspentAll)

	uiUtxoByScriptHash.Get("/:addressOrScripthash/stats", s.GetScriptStats)

	bulkByAddress.Post("/confirmed/unspent", s.PostBulkConfirmedUnspentByAddressOrByScript)
	bulkByScriptHash.Post("/confirmed/unspent", s.PostBulkConfirmedUnspentByAddressOrByScript)
	bulkByAddress.Post("/confirmed/history", s.PostBulkConfirmedHistoryByAddressOrByScript)
	bulkByScriptHash.Post("/confirmed/history", s.PostBulkConfirmedHistoryByAddressOrByScript)
	bulkByAddress.Post("/confirmed/balance", s.PostBulkConfirmedBalanceByAddressOrByScript)
	bulkByScriptHash.Post("/confirmed/balance", s.PostBulkConfirmedBalanceByAddressOrByScript)

	bulkByAddress.Post("/unconfirmed/unspent", s.PostBulkMempoolUnspentByAddressOrByScript)
	bulkByScriptHash.Post("/unconfirmed/unspent", s.PostBulkMempoolUnspentByAddressOrByScript)
	bulkByAddress.Post("/unconfirmed/history", s.PostBulkMempoolHistoryByAddressOrByScript)
	bulkByScriptHash.Post("/unconfirmed/history", s.PostBulkMempoolHistoryByAddressOrByScript)
	bulkByAddress.Post("/unconfirmed/balance", s.PostBulkMempoolBalanceByAddressOrByScript)
	bulkByScriptHash.Post("/unconfirmed/balance", s.PostBulkMempoolBalanceByAddressOrByScript)

	bulkByAddress.Post("/history/all", s.PostBulkHistoryByAddressOrByScript)
	bulkByScriptHash.Post("/history/all", s.PostBulkHistoryByAddressOrByScript)

	utxo.Post("/spent", s.PostBulkSpentIn)
	uiStats.Post("/query", s.StatsQuery)

	tx.Get("/:txid/:vout/unconfirmed/spent", s.GetUTXOMempoolSpendIn)
	tx.Get("/:txid/:vout/confirmed/spent", s.GetUTXOConfirmedSpendIn)
	tx.Get("/:txid/:vout/spent", s.GetUTXOSpendIn)
	tx.Get("/hash/:txid/propagation", s.TxPropagation)
	tx.Get("/:txid/beef", s.Beef)
	tx.Get("/:txid/proof/bump", s.Bump)

}
