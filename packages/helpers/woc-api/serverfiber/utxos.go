package serverfiber

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/ordishs/gocore"
	"github.com/teranode-group/common/logger"
	"github.com/teranode-group/common/utils"
	utxos_mempool "github.com/teranode-group/proto/utxos-mempool"
	"github.com/teranode-group/woc-api/utxosmempool"
	"github.com/teranode-group/woc-api/utxostore"
	"go.uber.org/zap"
)

type BulkAddressorScriptRequest struct {
	Addresses []string `json:"addresses"`
	Scripts   []string `json:"scripts"`
}

// GetMempoolResp represents the response to GetHistory() and GetMempool().
type HistoryResp struct {
	Address    string    `json:"address,omitempty"`
	Scripthash string    `json:"script,omitempty"`
	Result     []History `json:"result"`
	PageToken  string    `json:"nextPageToken,omitempty"`
	Error      string    `json:"error"`
}

type History struct {
	Hash   string `json:"tx_hash"`
	Height int32  `json:"height,omitempty"`
}

func (s *Server) GetAddressConfirmedHistory(c *fiber.Ctx) error {
	scriptHash := c.Params("addressOrScripthash")

	// limit
	limitStr := c.Query("limit", "100")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 10000 {
		limit = 100 //TODO: const
	}

	//afterheight
	heightStr := strings.ToLower(c.Query("height", "0"))
	height, err := strconv.ParseInt(heightStr, 10, 64)
	if err != nil || height < 0 {
		height = 0 //TODO: const
	}
	//nextPageToken
	pageTokenStr := strings.ToLower(c.Query("token", ""))

	// Covering an scenario where a user appending the token twice with ?token= which creates a invalid token.
	// We want to remove that part if it exists
	if strings.Contains(pageTokenStr, "?") {
		pageTokenStr = strings.Split(pageTokenStr, "?")[0]
	}

	//order
	orderStr := strings.ToLower(c.Query("order", "desc"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	}

	address := ""
	// if address convert to script hash
	if len(scriptHash) != 64 {
		address = scriptHash
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Log.Error("AddressToScriptHash request failure", zap.String("address", scriptHash), zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	// When the caller gave us an address, look up any scripthashes utxo-store
	// associates with it and pick the canonical one to query. This mirrors
	// GetAddressOrScripthashPage: prefer a "pubkeyhash" entry if present;
	// otherwise fall back to the first associated scripthash. Makes /history
	// return data for addresses whose only activity is under a non-standard
	// ("pubkey") scripthash.
	if len(address) > 0 {
		scripts, errScripts := utxostore.GetConfirmedScriptsByAddress(address)
		if errScripts != nil {
			errMsg := strings.ToLower(errScripts.Error())
			if !strings.Contains(errMsg, "unknown") && !strings.Contains(errMsg, "not found") {
				logger.Log.Error("GetConfirmedScriptsByAddress request failure", zap.String("address", address), zap.Error(errScripts))
			}
		} else if scripts != nil && len(scripts.ScripthashType) > 0 {
			picked := scripts.ScripthashType[0].Scripthash
			for _, s := range scripts.ScripthashType {
				if s.Type == "pubkeyhash" {
					picked = s.Scripthash
					break
				}
			}
			scriptHash = picked
		}
	}

	list, err := utxostore.GetConfirmedHistoryByScriptHash(scriptHash, int32(limit), pageTokenStr, uint32(height), order)
	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Log.Error("GetConfirmedHistoryByScriptHash request failure", zap.String("scriptHash", scriptHash), zap.String("pageTokenStr", pageTokenStr), zap.Error(err))
		}
		return c.SendStatus(fiber.StatusNotFound)
	}

	history := []History{}
	for _, tx := range list.ConfirmedTransactions {
		record := History{Hash: tx.GetTxId(), Height: int32(tx.BlockHeight)}
		history = append([]History{record}, history...)
	}

	resp := &HistoryResp{
		Scripthash: scriptHash,
		Address:    address,
		Result:     history,
		PageToken:  list.ConfirmedNextPageToken,
	}

	return c.JSON(resp)
}

func (s *Server) PostBulkConfirmedHistoryByAddressOrByScript(c *fiber.Ctx) error {

	b := new(BulkAddressorScriptRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	//order
	orderStr := strings.ToLower(c.Query("order", "desc"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	}

	containsScripts := false

	// list of address or scripthahes
	var items []string

	if b.Addresses != nil && len(b.Addresses) > 0 && len(b.Addresses) <= 20 {
		items = utils.Unique(b.Addresses)
	} else if b.Scripts != nil && len(b.Scripts) > 0 && len(b.Scripts) <= 20 {
		items = utils.Unique(b.Scripts)
		containsScripts = true
	} else {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]HistoryResp

	// default limit 20 per address
	var limit = 20

	for index, item := range items {

		scriptHash := item

		if !containsScripts {
			response[index].Address = item
			var err error
			scriptHash, err = utils.AddressToScriptHash(item, network)
			if err != nil {
				response[index].Error = "Unable to convert address to scripthash"
				continue
			}
		}

		response[index].Scripthash = scriptHash

		history := []History{}

		list, err := utxostore.GetConfirmedHistoryByScriptHash(scriptHash, int32(limit), "", 0, order)
		if err != nil && (!strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found")) {
			logger.Log.Error("GetConfirmedHistoryByScriptHash request failure", zap.Error(err))
			response[index].Error = "Failed to get History data"
			continue
		} else {
			if list != nil && len(list.ConfirmedTransactions) > 0 {
				for _, tx := range list.ConfirmedTransactions {
					record := History{Hash: tx.GetTxId(), Height: int32(tx.BlockHeight)}
					history = append([]History{record}, history...)
				}
				response[index].PageToken = list.ConfirmedNextPageToken
			}
		}
		response[index].Result = history
	}

	return c.JSON(response[0:len(items)])

}

// GetMempoolResp represents the response to GetHistory() and GetMempool().
type UnspentResp struct {
	Address    string    `json:"address,omitempty"`
	Scripthash string    `json:"script,omitempty"`
	Result     []Unspent `json:"result"`
	PageToken  string    `json:"nextPageToken,omitempty"`
	Error      string    `json:"error"`
}

type Unspent struct {
	Height           uint32 `json:"height,omitempty"`
	Position         uint32 `json:"tx_pos"`
	Hash             string `json:"tx_hash"`
	Value            int64  `json:"value"`
	IsSpentInMempool bool   `json:"isSpentInMempoolTx"`
	Hex              string `json:"hex,omitempty"`
	Status           string `json:"status,omitempty"`
}

func (s *Server) GetAddressConfirmedUnspent(c *fiber.Ctx) error {
	scriptHash := c.Params("addressOrScripthash")

	// limit
	limitStr := c.Query("limit", "1000")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 1000
	}

	//nextPageToken
	pageTokenStr := strings.ToLower(c.Query("token", ""))

	// Covering an scenario where a user appending the token twice with ?token= which creates a invalid token.
	// We want to remove that part if it exists
	if strings.Contains(pageTokenStr, "?") {
		pageTokenStr = strings.Split(pageTokenStr, "?")[0]
	}

	address := ""
	// if address convert to script hash
	if len(scriptHash) != 64 {
		address = scriptHash
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Log.Error("AddressToScriptHash request failure", zap.String("address", scriptHash), zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	list, err := utxostore.GetConfirmedUnspentByScriptHash(scriptHash, int32(limit), pageTokenStr)
	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Log.Error("GetConfirmedHistoryByScriptHash request failure", zap.Error(err))
			c.SendStatus(fiber.StatusInternalServerError)
		}
		return c.SendStatus(fiber.StatusNotFound)
	}

	txCount := len(list.UnspentTransactionsConfirmed)
	unspent := make([]Unspent, txCount)
	utxoStoreUnspentCheckInMempool := gocore.Config().GetBool("utxoStoreUnspentCheckInMempool", true)

	for i, tx := range list.UnspentTransactionsConfirmed {
		unspent[i] = Unspent{Hash: tx.GetTxId(), Position: tx.Position, Height: uint32(tx.BlockHeight), Value: tx.Satoshis}
	}

	if utxoStoreUnspentCheckInMempool && txCount > 0 {
		// Batch check mempool spent status in one gRPC call
		items := make([]*utxos_mempool.GetSpentTransactionRequest, txCount)
		for i, tx := range list.UnspentTransactionsConfirmed {
			items[i] = &utxos_mempool.GetSpentTransactionRequest{TxId: tx.GetTxId(), Vout: uint32(tx.Position)}
		}
		results, err := utxosmempool.GetBatchedMempoolSpentIn(items)
		if err == nil && len(results) == txCount {
			for i, r := range results {
				if r.IsSpent {
					unspent[i].IsSpentInMempool = true
				}
			}
		}
	}

	// Reverse to match previous order (newest first)
	for i, j := 0, len(unspent)-1; i < j; i, j = i+1, j-1 {
		unspent[i], unspent[j] = unspent[j], unspent[i]
	}

	resp := &UnspentResp{
		Scripthash: scriptHash,
		Address:    address,
		Result:     unspent,
		PageToken:  list.NextPageToken,
	}

	return c.JSON(resp)
}

func (s *Server) PostBulkConfirmedUnspentByAddressOrByScript(c *fiber.Ctx) error {

	b := new(BulkAddressorScriptRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	containsScripts := false

	// list of address or scripthahes
	var items []string

	if b.Addresses != nil && len(b.Addresses) > 0 && len(b.Addresses) <= 20 {
		items = utils.Unique(b.Addresses)
	} else if b.Scripts != nil && len(b.Scripts) > 0 && len(b.Scripts) <= 20 {
		items = utils.Unique(b.Scripts)
		containsScripts = true
	} else {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]UnspentResp

	// default limit 20 per address
	var limit = 20

	for index, item := range items {

		scriptHash := item

		if !containsScripts {
			response[index].Address = item
			var err error
			scriptHash, err = utils.AddressToScriptHash(item, network)
			if err != nil {
				response[index].Error = "Unable to convert address to scripthash"
				continue
			}
		}

		response[index].Scripthash = scriptHash

		unspent := []Unspent{}

		list, err := utxostore.GetConfirmedUnspentByScriptHash(scriptHash, int32(limit), "")
		if err != nil && (!strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found")) {
			logger.Log.Error("GetConfirmedUnspentByScriptHash request failure", zap.Error(err))
			response[index].Error = "Failed to get Unspent data"
			continue
		} else {

			if list != nil && len(list.UnspentTransactionsConfirmed) > 0 {
				utxoStoreUnspentCheckInMempool := gocore.Config().GetBool("utxoStoreUnspentCheckInMempool", true)
				unspent = make([]Unspent, 0, len(list.UnspentTransactionsConfirmed))
				for _, tx := range list.UnspentTransactionsConfirmed {
					isSpent := false

					//check if spent in the mempool
					if utxoStoreUnspentCheckInMempool {
						spentIn, _ := utxosmempool.GetMempoolSpentInByTxIdOut(tx.GetTxId(), uint32(tx.Position))
						if spentIn != nil {
							isSpent = true
						}
					}

					record := Unspent{Hash: tx.GetTxId(), Position: tx.Position, Height: uint32(tx.BlockHeight), Value: tx.Satoshis, IsSpentInMempool: isSpent}
					unspent = append(unspent, record)
				}
				// Reverse to match previous order
				for i, j := 0, len(unspent)-1; i < j; i, j = i+1, j-1 {
					unspent[i], unspent[j] = unspent[j], unspent[i]
				}
				response[index].PageToken = list.NextPageToken
			}
		}

		response[index].Result = unspent
	}

	return c.JSON(response[0:len(items)])

}

type BalanceResp struct {
	Address    string             `json:"address,omitempty"`
	Scripthash string             `json:"script,omitempty"`
	Confirmed  uint64             `json:"confirmed"`
	Error      string             `json:"error"`
	Scripts    []AssociatedScript `json:"associatedScripts,omitempty"`
}

type AssociatedScript struct {
	Scripthash string `json:"script,omitempty"`
	Type       string `json:"type,omitempty"`
}

func (s *Server) GetAddressConfirmedBalance(c *fiber.Ctx) error {
	scriptHash := c.Params("addressOrScripthash")

	var err error

	address := ""
	// if address convert to script hash
	if len(scriptHash) != 64 {
		address = scriptHash
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Log.Error("AddressToScriptHash request failure", zap.String("address", scriptHash), zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	resp := &BalanceResp{
		Scripthash: scriptHash,
		Address:    address,
	}

	utxoBalance, err := utxostore.GetBalancePITB(scriptHash)
	if err != nil {
		logger.Log.Error("GetAddressConfirmedBalance request failure", zap.Error(err))
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	resp.Scripts = []AssociatedScript{}

	if utxoBalance.Unspent != nil {
		resp.Confirmed = uint64(utxoBalance.Unspent.Satoshis)

		if len(address) > 0 {
			//get associated scripts
			scripts, err := utxostore.GetConfirmedScriptsByAddress(address)
			if err != nil {
				errMsg := strings.ToLower(err.Error())
				if !strings.Contains(errMsg, "unknown") {
					logger.Log.Error("error: Couldn't get scripts by address", zap.String("address", address), zap.Error(err))
				}
			} else if scripts != nil && scripts.ScripthashType != nil {

				for _, s := range scripts.ScripthashType {

					if s.Scripthash != "multisig" {
						resp.Scripts = append(resp.Scripts, AssociatedScript{Scripthash: s.Scripthash, Type: s.Type})

						otherBalance, errBalance := utxostore.GetBalancePITB(s.Scripthash)
						if errBalance != nil {
							logger.Log.Error("error: Couldn't get utxo store  GetBalancePITB balance for scripthash",
								zap.String("scripthash", address), zap.Error(err))
						} else if s.Scripthash != scriptHash {
							if otherBalance != nil && otherBalance.Unspent != nil {
								resp.Confirmed = resp.Confirmed + uint64(otherBalance.Unspent.Satoshis)
							}
						}

					}
				}

			}

		}

		return c.JSON(resp)
	}

	return c.SendStatus(fiber.StatusNotFound)
}

func (s *Server) PostBulkConfirmedBalanceByAddressOrByScript(c *fiber.Ctx) error {

	b := new(BulkAddressorScriptRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	containsScripts := false

	// list of address or scripthahes
	var items []string

	if b.Addresses != nil && len(b.Addresses) > 0 && len(b.Addresses) <= 20 {
		items = utils.Unique(b.Addresses)
	} else if b.Scripts != nil && len(b.Scripts) > 0 && len(b.Scripts) <= 20 {
		items = utils.Unique(b.Scripts)
		containsScripts = true
	} else {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]BalanceResp

	for index, item := range items {

		scriptHash := item
		address := ""

		if !containsScripts {
			response[index].Address = item
			address = item
			var err error
			scriptHash, err = utils.AddressToScriptHash(item, network)
			if err != nil {
				response[index].Error = "Unable to convert address to scripthash"
				continue
			}
		}

		response[index].Scripthash = scriptHash

		// Build the list of scripthashes to query. Always include the standard
		// scripthash; if the caller gave us an address, also include any other
		// scripthashes utxo-store associates with it (pubkey-derived, etc.).
		scriptHashes := []string{scriptHash}
		if len(address) > 0 {
			scripts, errScripts := utxostore.GetConfirmedScriptsByAddress(address)
			if errScripts != nil {
				errMsg := strings.ToLower(errScripts.Error())
				if !strings.Contains(errMsg, "unknown") && !strings.Contains(errMsg, "not found") {
					logger.Log.Error("GetConfirmedScriptsByAddress request failure", zap.String("address", address), zap.Error(errScripts))
				}
			} else if scripts != nil && scripts.ScripthashType != nil {
				seen := map[string]bool{scriptHash: true}
				for _, sh := range scripts.ScripthashType {
					if sh.Scripthash != "multisig" && !seen[sh.Scripthash] {
						scriptHashes = append(scriptHashes, sh.Scripthash)
						seen[sh.Scripthash] = true
					}
				}
			}
		}

		var utxoStoreConfirmed int64 = 0
		var firstErr error
		for _, sh := range scriptHashes {
			utxoBalance, err := utxostore.GetBalanceByScriptHash(sh)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if utxoBalance.Unspent != nil {
				utxoStoreConfirmed += utxoBalance.Unspent.Satoshis
			}
		}
		// If every scripthash failed, surface an error.
		if utxoStoreConfirmed == 0 && firstErr != nil {
			logger.Log.Error("PostBulkConfirmedBalance: all scripthash queries failed", zap.String("address", address), zap.Error(firstErr))
			response[index].Error = "Failed to get balance data"
			continue
		}

		response[index].Confirmed = uint64(utxoStoreConfirmed)

	}

	return c.JSON(response[0:len(items)])

}

func (s *Server) GetAddressUnspentAll(c *fiber.Ctx) error {
	scriptHash := c.Params("addressOrScripthash")

	// limit
	limitStr := c.Query("limit", "1000")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 1000
	}

	//debug
	debugMode, _ := strconv.ParseBool(c.Query("debug", ""))

	//nextPageToken
	pageTokenStr := strings.ToLower(c.Query("token", ""))

	// Covering an scenario where a user appending the token twice with ?token= which creates a invalid token.
	// We want to remove that part if it exists
	if strings.Contains(pageTokenStr, "?") {
		pageTokenStr = strings.Split(pageTokenStr, "?")[0]
	}

	address := ""

	// if address convert to script hash
	if len(scriptHash) != 64 {
		address = scriptHash
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Log.Error("AddressToScriptHash request failure", zap.String("address", scriptHash), zap.Error(err))
			return c.SendStatus(fiber.StatusBadRequest)
		}
	}

	unspentInMempool := []Unspent{}

	//Get mempool utxos if page token is not provided
	if len(pageTokenStr) == 0 {
		mempoolLimit := 100000
		mempoolList, err := utxosmempool.GetMempoolUnspentByScriptHash(scriptHash, int32(mempoolLimit), pageTokenStr, debugMode)
		if err != nil {
			if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") &&
				!strings.Contains(err.Error(), "Unknown") && !strings.Contains(err.Error(), "unknown") {

				logger.Log.Error("GetMempoolUnspentByScriptHash request failure", zap.Error(err))
				//doen't return try confirmed
			}
		} else {
			unspentInMempool = make([]Unspent, 0, len(mempoolList.UnspentTransactionsMempool))
			for _, tx := range mempoolList.UnspentTransactionsMempool {
				record := Unspent{Hash: tx.GetTxId(), Position: tx.Position, Value: tx.Satoshis, Hex: tx.LockingScriptHex, Status: "unconfirmed"}
				unspentInMempool = append(unspentInMempool, record)
			}
			// Reverse to match previous order
			for i, j := 0, len(unspentInMempool)-1; i < j; i, j = i+1, j-1 {
				unspentInMempool[i], unspentInMempool[j] = unspentInMempool[j], unspentInMempool[i]
			}
		}
	}
	//Get confirmed utxos
	unspentInBlocks := []Unspent{}

	list, err := utxostore.GetConfirmedUnspentByScriptHash(scriptHash, int32(limit), pageTokenStr)
	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") &&
			!strings.Contains(err.Error(), "Unknown") && !strings.Contains(err.Error(), "unknown") {

			logger.Log.Error("GetConfirmedHistoryByScriptHash request failure", zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	} else {
		txCount := len(list.UnspentTransactionsConfirmed)
		unspentInBlocks = make([]Unspent, txCount)
		utxoStoreUnspentCheckInMempool := gocore.Config().GetBool("utxoStoreUnspentCheckInMempool", true)

		for i, tx := range list.UnspentTransactionsConfirmed {
			unspentInBlocks[i] = Unspent{Hash: tx.GetTxId(), Position: tx.Position, Height: tx.BlockHeight, Value: tx.Satoshis, Status: "confirmed"}
		}

		if utxoStoreUnspentCheckInMempool && txCount > 0 {
			items := make([]*utxos_mempool.GetSpentTransactionRequest, txCount)
			for i, tx := range list.UnspentTransactionsConfirmed {
				items[i] = &utxos_mempool.GetSpentTransactionRequest{TxId: tx.GetTxId(), Vout: tx.Position}
			}
			results, err := utxosmempool.GetBatchedMempoolSpentIn(items)
			if err == nil && len(results) == txCount {
				for i, r := range results {
					if r.IsSpent {
						unspentInBlocks[i].IsSpentInMempool = true
					}
				}
			}
		}

		// Reverse to match previous order
		for i, j := 0, len(unspentInBlocks)-1; i < j; i, j = i+1, j-1 {
			unspentInBlocks[i], unspentInBlocks[j] = unspentInBlocks[j], unspentInBlocks[i]
		}
	}

	unspent := append(unspentInMempool, unspentInBlocks...)

	pageToken := ""
	if list != nil && len(list.NextPageToken) > 0 {
		pageToken = list.NextPageToken
	}

	resp := &UnspentResp{
		Scripthash: scriptHash,
		Address:    address,
		Result:     unspent,
		PageToken:  pageToken,
	}

	return c.JSON(resp)
}

// ***** utxos-mempool *****

func (s *Server) GetAddressMempoolHistory(c *fiber.Ctx) error {
	scriptHash := c.Params("addressOrScripthash")

	// limit
	limitStr := c.Query("limit", "100000")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 100000 {
		limit = 100000 //TODO: const
	}

	//nextPageToken
	pageTokenStr := strings.ToLower(c.Query("token", ""))

	// Covering an scenario where a user appending the token twice with ?token= which creates a invalid token.
	// We want to remove that part if it exists
	if strings.Contains(pageTokenStr, "?") {
		pageTokenStr = strings.Split(pageTokenStr, "?")[0]
	}

	//order
	orderStr := strings.ToLower(c.Query("order", "desc"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	}

	address := ""
	// if address convert to script hash
	if len(scriptHash) != 64 {
		address = scriptHash
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Log.Error("AddressToScriptHash request failure", zap.String("address", scriptHash), zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}
	list, err := utxosmempool.GetMempoolHistoryByScriptHash(scriptHash, int32(limit), pageTokenStr, uint32(0), order)
	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Log.Error("GetConfirmedHistoryByScriptHash request failure", zap.Error(err))
		}
		return c.SendStatus(fiber.StatusNotFound)
	}

	history := []History{}
	for _, tx := range list.MempoolTransactions {
		record := History{Hash: tx.GetTxId()}
		history = append([]History{record}, history...)
	}

	resp := &HistoryResp{
		Scripthash: scriptHash,
		Address:    address,
		Result:     history,
		PageToken:  list.NextPageToken,
	}

	return c.JSON(resp)
}

func (s *Server) PostBulkMempoolHistoryByAddressOrByScript(c *fiber.Ctx) error {

	b := new(BulkAddressorScriptRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	//order
	orderStr := strings.ToLower(c.Query("order", "desc"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	}

	containsScripts := false

	// list of address or scripthahes
	var items []string

	if b.Addresses != nil && len(b.Addresses) > 0 && len(b.Addresses) <= 20 {
		items = utils.Unique(b.Addresses)
	} else if b.Scripts != nil && len(b.Scripts) > 0 && len(b.Scripts) <= 20 {
		items = utils.Unique(b.Scripts)
		containsScripts = true
	} else {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]HistoryResp

	// default limit 20 per address
	var limit = 20

	for index, item := range items {

		scriptHash := item

		if !containsScripts {
			response[index].Address = item
			var err error
			scriptHash, err = utils.AddressToScriptHash(item, network)
			if err != nil {
				response[index].Error = "Unable to convert address to scripthash"
				continue
			}
		}

		response[index].Scripthash = scriptHash

		history := []History{}

		list, err := utxosmempool.GetMempoolHistoryByScriptHash(scriptHash, int32(limit), "", uint32(0), order)
		if err != nil && (!strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found")) {
			logger.Log.Error("GetMempoolHistoryByScriptHash request failure", zap.Error(err))
			response[index].Error = "Failed to get History data"
			continue
		} else {
			if list != nil && len(list.MempoolTransactions) > 0 {
				for _, tx := range list.MempoolTransactions {
					record := History{Hash: tx.GetTxId()}
					history = append([]History{record}, history...)
				}
				response[index].PageToken = list.NextPageToken
			}
		}
		response[index].Result = history
	}

	return c.JSON(response[0:len(items)])

}

func (s *Server) GetAddressMempoolUnspent(c *fiber.Ctx) error {
	scriptHash := c.Params("addressOrScripthash")

	// limit
	limitStr := c.Query("limit", "100000")
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit < 1 || limit > 100000 {
		limit = 100000
	}

	//nextPageToken
	pageTokenStr := strings.ToLower(c.Query("token", ""))

	// Covering an scenario where a user appending the token twice with ?token= which creates a invalid token.
	// We want to remove that part if it exists
	if strings.Contains(pageTokenStr, "?") {
		pageTokenStr = strings.Split(pageTokenStr, "?")[0]
	}

	//debug
	debugMode, _ := strconv.ParseBool(c.Query("debug", ""))

	address := ""
	// if address convert to script hash
	if len(scriptHash) != 64 {
		address = scriptHash
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Log.Error("AddressToScriptHash request failure", zap.String("address", scriptHash), zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	list, err := utxosmempool.GetMempoolUnspentByScriptHash(scriptHash, int32(limit), pageTokenStr, debugMode)
	if err != nil {
		if !strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found") {
			logger.Log.Error("GetConfirmedHistoryByScriptHash request failure", zap.Error(err))
		}
		return c.SendStatus(fiber.StatusNotFound)
	}

	unspent := make([]Unspent, 0, len(list.UnspentTransactionsMempool))

	for _, tx := range list.UnspentTransactionsMempool {
		record := Unspent{Hash: tx.GetTxId(), Position: tx.Position, Value: tx.Satoshis, Hex: tx.LockingScriptHex}
		unspent = append(unspent, record)
	}

	// Reverse to match previous order
	for i, j := 0, len(unspent)-1; i < j; i, j = i+1, j-1 {
		unspent[i], unspent[j] = unspent[j], unspent[i]
	}

	resp := &UnspentResp{
		Scripthash: scriptHash,
		Address:    address,
		Result:     unspent,
		PageToken:  list.NextPageToken,
	}

	return c.JSON(resp)
}

func (s *Server) PostBulkMempoolUnspentByAddressOrByScript(c *fiber.Ctx) error {

	b := new(BulkAddressorScriptRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	containsScripts := false

	// list of address or scripthahes
	var items []string

	if b.Addresses != nil && len(b.Addresses) > 0 && len(b.Addresses) <= 20 {
		items = utils.Unique(b.Addresses)
	} else if b.Scripts != nil && len(b.Scripts) > 0 && len(b.Scripts) <= 20 {
		items = utils.Unique(b.Scripts)
		containsScripts = true
	} else {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]UnspentResp

	// default limit 20 per address
	var limit = 100

	for index, item := range items {

		scriptHash := item

		if !containsScripts {
			response[index].Address = item
			var err error
			scriptHash, err = utils.AddressToScriptHash(item, network)
			if err != nil {
				response[index].Error = "Unable to convert address to scripthash"
				continue
			}
		}

		response[index].Scripthash = scriptHash

		unspent := []Unspent{}

		list, err := utxosmempool.GetMempoolUnspentByScriptHash(scriptHash, int32(limit), "", false)
		if err != nil && (!strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found")) {
			logger.Log.Error("GetMempoolUnspentByScriptHash request failure", zap.Error(err))
			response[index].Error = "Failed to get Unspent data"
			continue
		} else {

			if list != nil && len(list.UnspentTransactionsMempool) > 0 {
				unspent = make([]Unspent, 0, len(list.UnspentTransactionsMempool))
				for _, tx := range list.UnspentTransactionsMempool {
					record := Unspent{Hash: tx.GetTxId(), Position: tx.Position, Value: tx.Satoshis, Hex: tx.LockingScriptHex}
					unspent = append(unspent, record)
				}
				// Reverse to match previous order
				for i, j := 0, len(unspent)-1; i < j; i, j = i+1, j-1 {
					unspent[i], unspent[j] = unspent[j], unspent[i]
				}
				response[index].PageToken = list.NextPageToken
			}
		}

		response[index].Result = unspent
	}

	return c.JSON(response[0:len(items)])

}

type BalanceMempoolResp struct {
	Address     string `json:"address,omitempty"`
	Scripthash  string `json:"script,omitempty"`
	Unconfirmed int64  `json:"unconfirmed"`
	Error       string `json:"error"`
}

func (s *Server) GetAddressMempoolBalance(c *fiber.Ctx) error {
	scriptHash := c.Params("addressOrScripthash")

	var err error

	address := ""
	// if address convert to script hash
	if len(scriptHash) != 64 {
		address = scriptHash
		scriptHash, err = utils.AddressToScriptHash(scriptHash, network)
		if err != nil {
			logger.Log.Error("AddressToScriptHash request failure", zap.String("address", scriptHash), zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	utxoBalance, err := utxosmempool.GetMempoolBalanceByScriptHash(scriptHash)
	if err != nil {
		logger.Log.Error("GetAddressUnconfirmedBalance request failure", zap.Error(err))
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	var mempoolBalance int64 = 0

	if utxoBalance.ScripthashMempool != nil && utxoBalance.ScripthashMempool.Unspent != nil {
		mempoolBalance = utxoBalance.ScripthashMempool.Unspent.Satoshis

		resp := &BalanceMempoolResp{
			Scripthash:  scriptHash,
			Address:     address,
			Unconfirmed: mempoolBalance,
		}

		return c.JSON(resp)
	}

	return c.SendStatus(fiber.StatusNotFound)
}

func (s *Server) PostBulkMempoolBalanceByAddressOrByScript(c *fiber.Ctx) error {

	b := new(BulkAddressorScriptRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	containsScripts := false

	// list of address or scripthahes
	var items []string

	if b.Addresses != nil && len(b.Addresses) > 0 && len(b.Addresses) <= 20 {
		items = utils.Unique(b.Addresses)
	} else if b.Scripts != nil && len(b.Scripts) > 0 && len(b.Scripts) <= 20 {
		items = utils.Unique(b.Scripts)
		containsScripts = true
	} else {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]BalanceMempoolResp

	for index, item := range items {

		scriptHash := item

		if !containsScripts {
			response[index].Address = item
			var err error
			scriptHash, err = utils.AddressToScriptHash(item, network)
			if err != nil {
				response[index].Error = "Unable to convert address to scripthash"
				continue
			}
		}

		response[index].Scripthash = scriptHash

		utxoBalance, err := utxosmempool.GetMempoolBalanceByScriptHash(scriptHash)
		if err != nil {
			logger.Log.Error("GetMempoolBalanceByScriptHash request failure", zap.Error(err))
			response[index].Error = "Failed to get balance data"
			continue
		}

		var utxoStoreUnconfirmed int64 = 0

		if utxoBalance.ScripthashMempool != nil && utxoBalance.ScripthashMempool.Unspent != nil {
			utxoStoreUnconfirmed = utxoBalance.ScripthashMempool.Unspent.Satoshis
		}

		response[index].Unconfirmed = utxoStoreUnconfirmed

	}

	return c.JSON(response[0:len(items)])

}

// /Spent in endpointss

type SpentInReq struct {
	TxId string `json:"txid"`
	Vout uint32 `json:"vout"`
}

type SpentInResp struct {
	TxId   string `json:"txid"`
	Vin    uint32 `json:"vin"`
	Status string `json:"status,omitempty"`
}

type BulkUtxosSpentInRequest struct {
	Utxos []SpentInReq `json:"utxos"`
}

type BulkUtxosSpentInItem struct {
	Utxo    SpentInReq   `json:"utxo"`
	SpentIn *SpentInResp `json:"spentIn,omitempty"`
	Error   string       `json:"error"`
}

func (s *Server) GetUTXOMempoolSpendIn(c *fiber.Ctx) error {
	txid := c.Params("txid")
	voutStr := c.Params("vout")

	vout, err := strconv.ParseInt(voutStr, 10, 64)
	if err != nil || len(txid) != 64 {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	spentIn, err := utxosmempool.GetMempoolSpentInByTxIdOut(txid, uint32(vout))
	if err != nil {
		if strings.Contains(err.Error(), "Unknown") && strings.Contains(err.Error(), "unknown") {
			return c.SendStatus(fiber.StatusBadRequest)
		} else if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
			logger.Log.Error("GetMempoolSpentInByTxIdVout request failure", zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	if spentIn != nil {

		resp := &SpentInResp{
			TxId:   spentIn.TxId,
			Vin:    spentIn.Vin,
			Status: "unconfirmed",
		}

		return c.JSON(resp)
	}

	return c.SendStatus(fiber.StatusNotFound)
}

func (s *Server) GetUTXOConfirmedSpendIn(c *fiber.Ctx) error {
	txid := c.Params("txid")
	voutStr := c.Params("vout")

	vout, err := strconv.ParseInt(voutStr, 10, 64)
	if err != nil || len(txid) != 64 {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	spentIn, err := utxostore.GetConfirmedSpentInByTxIdOut(txid, uint32(vout))
	if err != nil {
		if strings.Contains(err.Error(), "Unknown") && strings.Contains(err.Error(), "unknown") {
			return c.SendStatus(fiber.StatusBadRequest)
		} else if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
			logger.Log.Error("GetMempoolSpentInByTxIdVout request failure", zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	if spentIn != nil {

		resp := &SpentInResp{
			TxId:   spentIn.TxId,
			Vin:    spentIn.Vin,
			Status: "confirmed",
		}

		return c.JSON(resp)
	}

	return c.SendStatus(fiber.StatusNotFound)
}

func (s *Server) GetUTXOSpendIn(c *fiber.Ctx) error {
	txid := c.Params("txid")
	voutStr := c.Params("vout")

	unknownInConfirmed := false
	unknownInMemepool := false

	vout, err := strconv.ParseInt(voutStr, 10, 64)
	if err != nil || len(txid) != 64 {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	//add spent details
	confirmedSpentIn, err := utxostore.GetConfirmedSpentInByTxIdOut(txid, uint32(vout))
	if err == nil && confirmedSpentIn != nil {
		resp := &SpentInResp{
			TxId:   confirmedSpentIn.TxId,
			Vin:    confirmedSpentIn.Vin,
			Status: "confirmed",
		}
		return c.JSON(resp)
	}

	if err != nil {
		if strings.Contains(err.Error(), "Unknown") && strings.Contains(err.Error(), "unknown") {
			unknownInConfirmed = true
		} else if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
			logger.Log.Error("GetAddressConfirmedorMempoolSpendIn: GetConfirmedSpentInByTxIdOut request failure", zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	mempoolSpentIn, err := utxosmempool.GetMempoolSpentInByTxIdOut(txid, uint32(vout))
	if err == nil && mempoolSpentIn != nil {
		resp := &SpentInResp{
			TxId:   mempoolSpentIn.TxId,
			Vin:    mempoolSpentIn.Vin,
			Status: "unconfirmed",
		}
		return c.JSON(resp)
	}

	if err != nil {
		if strings.Contains(err.Error(), "Unknown") && strings.Contains(err.Error(), "unknown") {
			unknownInMemepool = true
		} else if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
			logger.Log.Error("GetAddressConfirmedorMempoolSpendIn: GetMempoolSpentInByTxIdOut request failure", zap.Error(err))
			return c.SendStatus(fiber.StatusInternalServerError)
		}
	}

	if unknownInConfirmed && unknownInMemepool {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	return c.SendStatus(fiber.StatusNotFound)
}

func (s *Server) PostBulkSpentIn(c *fiber.Ctx) error {

	b := new(BulkUtxosSpentInRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	if len(b.Utxos) > 20 {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]BulkUtxosSpentInItem

	for index, item := range b.Utxos {

		response[index].Utxo = item

		unknownInConfirmed := false
		unknownInMemepool := false

		if len(item.TxId) != 64 {
			response[index].Error = "Invalid txid"
			continue
		}

		//add spent details
		confirmedSpentIn, err := utxostore.GetConfirmedSpentInByTxIdOut(item.TxId, uint32(item.Vout))
		if err == nil && confirmedSpentIn != nil {
			response[index].SpentIn = &SpentInResp{
				TxId:   confirmedSpentIn.TxId,
				Vin:    confirmedSpentIn.Vin,
				Status: "confirmed",
			}
			continue
		}

		if err != nil {
			if strings.Contains(err.Error(), "Unknown") && strings.Contains(err.Error(), "unknown") {
				unknownInConfirmed = true
			} else if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
				logger.Log.Error("PostBulkSpentIn: GetConfirmedSpentInByTxIdOut request failure", zap.Error(err))
				response[index].Error = "Unable to get spent info"
				continue
			}
		}

		mempoolSpentIn, err := utxosmempool.GetMempoolSpentInByTxIdOut(item.TxId, item.Vout)
		if err == nil && mempoolSpentIn != nil {
			response[index].SpentIn = &SpentInResp{
				TxId:   mempoolSpentIn.TxId,
				Vin:    mempoolSpentIn.Vin,
				Status: "unconfirmed",
			}
			continue
		}

		if err != nil {
			if strings.Contains(err.Error(), "Unknown") && strings.Contains(err.Error(), "unknown") {
				unknownInMemepool = true
			} else if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
				logger.Log.Error("PostBulkSpentIn: GetMempoolSpentInByTxIdOut request failure", zap.Error(err))
				response[index].Error = "Unable to get spent info"
				continue
			}
		}

		if unknownInConfirmed && unknownInMemepool {
			response[index].SpentIn = &SpentInResp{
				TxId:   item.TxId,
				Vin:    item.Vout,
				Status: "Unknown UTXO",
			}
		}

	}

	return c.JSON(response[0:len(b.Utxos)])

}

type HistoryCombinedItem struct {
	Result    []History `json:"result"`
	PageToken string    `json:"nextPageToken,omitempty"`
	Error     string    `json:"error"`
}

type HistoryCombined struct {
	Address     string              `json:"address,omitempty"`
	Scripthash  string              `json:"script,omitempty"`
	Unconfirmed HistoryCombinedItem `json:"unconfirmed"`
	Confirmed   HistoryCombinedItem `json:"confirmed"`
}

func (s *Server) PostBulkHistoryByAddressOrByScript(c *fiber.Ctx) error {

	b := new(BulkAddressorScriptRequest)

	if err := c.BodyParser(b); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	//order
	orderStr := strings.ToLower(c.Query("order", "desc"))

	// Default order desc
	order := 1
	if orderStr == "asc" {
		order = 0
	}

	containsScripts := false

	// list of address or scripthahes
	var items []string

	if b.Addresses != nil && len(b.Addresses) > 0 && len(b.Addresses) <= 20 {
		items = utils.Unique(b.Addresses)
	} else if b.Scripts != nil && len(b.Scripts) > 0 && len(b.Scripts) <= 20 {
		items = utils.Unique(b.Scripts)
		containsScripts = true
	} else {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	var response [20]HistoryCombined

	// default limit 1000 per address
	var limit = 1000

	for index, item := range items {

		response[index].Unconfirmed = HistoryCombinedItem{}
		response[index].Confirmed = HistoryCombinedItem{}

		scriptHash := item

		if !containsScripts {
			response[index].Address = item
			var err error
			scriptHash, err = utils.AddressToScriptHash(item, network)
			if err != nil {
				response[index].Unconfirmed.Error = "Unable to convert address to scripthash"
				response[index].Confirmed.Error = "Unable to convert address to scripthash"
				continue
			}
		}

		response[index].Scripthash = scriptHash

		// Mempool
		history := []History{}

		list, err := utxosmempool.GetMempoolHistoryByScriptHash(scriptHash, int32(limit), "", uint32(0), order)
		if err != nil && (!strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found")) {
			logger.Log.Error("GetMempoolHistoryByScriptHash request failure", zap.Error(err))
			response[index].Unconfirmed.Error = "Failed to get History data"
			continue
		} else {
			if list != nil && len(list.MempoolTransactions) > 0 {
				for _, tx := range list.MempoolTransactions {
					record := History{Hash: tx.GetTxId()}
					history = append([]History{record}, history...)
				}
				response[index].Unconfirmed.PageToken = list.NextPageToken
			}
		}
		response[index].Unconfirmed.Result = history

		// Confirmed
		history = []History{}

		listConfirmed, err := utxostore.GetConfirmedHistoryByScriptHash(scriptHash, int32(limit), "", 0, order)
		if err != nil && (!strings.Contains(err.Error(), "NotFound") && !strings.Contains(err.Error(), "not found")) {
			logger.Log.Error("GetConfirmedHistoryByScriptHash request failure", zap.Error(err))
			response[index].Confirmed.Error = "Failed to get History data"
			continue
		} else {
			if listConfirmed != nil && len(listConfirmed.ConfirmedTransactions) > 0 {
				for _, tx := range listConfirmed.ConfirmedTransactions {
					record := History{Hash: tx.GetTxId(), Height: int32(tx.BlockHeight)}
					history = append([]History{record}, history...)
				}
				response[index].Confirmed.PageToken = listConfirmed.ConfirmedNextPageToken
			}
		}
		response[index].Confirmed.Result = history

	}

	return c.JSON(response[0:len(items)])

}

func (s *Server) IsAdressInUse(c *fiber.Ctx) error {
	address := c.Params("address")

	//add spent details
	scripts, err := utxostore.GetConfirmedScriptsByAddress(address)

	if err == nil && scripts != nil {
		return c.SendString("true")
	}

	memScripts, err := utxosmempool.GetMempoolScriptsByAddress(address)
	if err == nil && memScripts != nil {
		return c.SendString("true")
	}

	return c.SendStatus(fiber.StatusNotFound)
}

type AddressToScripts struct {
	Scripthash string `json:"script"`
	Type       string `json:"type"`
}

func getAddressScriptResponse(address string) []AddressToScripts {
	scriptsMap := make(map[string]string)
	var response []AddressToScripts

	//check utxo store
	scripts, err := utxostore.GetConfirmedScriptsByAddress(address)
	if err == nil && scripts != nil && scripts.ScripthashType != nil {
		for _, item := range scripts.ScripthashType {
			_, ok := scriptsMap[item.Scripthash]
			if !ok {
				scriptsMap[item.Scripthash] = item.Type
			}

		}

	}
	// check utxos-mempool
	memScripts, _ := utxosmempool.GetMempoolScriptsByAddress(address)
	if memScripts != nil {
		for _, item := range memScripts.ScripthashType {
			_, ok := scriptsMap[item.Scripthash]
			if !ok {
				scriptsMap[item.Scripthash] = item.Type
			}
		}
	}

	//merge confirmed and unconfirmed
	for k := range scriptsMap {
		response = append(response, AddressToScripts{Scripthash: k, Type: scriptsMap[k]})
	}

	return response
}

func (s *Server) GetAddressScripts(c *fiber.Ctx) error {
	address := c.Params("address")

	response := getAddressScriptResponse((address))
	if len(response) == 0 {
		return c.SendStatus(fiber.StatusNotFound)
	}

	return c.JSON(response)
}

// Mempool Stats from utxos-mempool for UI
type MempoolStatsTxFees struct {
	Enabled         bool     `json:"enabled"`
	TotalTxFeeCount uint64   `json:"total_tx_fee_count"`
	BucketsCount    []uint64 `json:"buckets_count"`
	BucketsList     []uint64 `json:"buckets_list"`
}

type MempoolStatsTxSizes struct {
	Enabled          bool     `json:"enabled"`
	TotalTxCount     uint64   `json:"total_tx_count"`
	TotalTxSizeBytes uint64   `json:"total_tx_size_bytes"`
	BucketsCount     []uint64 `json:"buckets_count"`
	BucketsList      []uint64 `json:"buckets_list"`
}

type MempoolStatsTxTags struct {
	Enabled              bool     `json:"enabled"`
	TotalTagsCount       uint64   `json:"total_tags_count"`
	TotalMissedTagsCount uint64   `json:"total_missed_tags_count"`
	BucketsCount         []uint64 `json:"buckets_count"`
	BucketsList          []string `json:"buckets_list"`
}

type MempoolStatsResp struct {
	TxfeesStats  MempoolStatsTxFees  `json:"tx_fees_stats"`
	TxSizesStats MempoolStatsTxSizes `json:"tx_sizes_stats"`
	TxTagsStats  MempoolStatsTxTags  `json:"tx_tags_stats"`
}

func (s *Server) GetMempoolStats(c *fiber.Ctx) error {

	stats, err := utxosmempool.GetMempoolStats()
	if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "NotFound") {
		logger.Log.Error("GetMempoolStats request failure", zap.Error(err))
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	if stats != nil {
		resp := &MempoolStatsResp{
			TxfeesStats: MempoolStatsTxFees{
				Enabled:         stats.Stats.Fees.Enabled,
				BucketsCount:    stats.Stats.Fees.BucketsCount,
				BucketsList:     stats.Stats.Fees.BucketsList,
				TotalTxFeeCount: stats.Stats.Fees.TotalTxFeeCount,
			},
			TxSizesStats: MempoolStatsTxSizes{
				Enabled:          stats.Stats.Sizes.Enabled,
				BucketsCount:     stats.Stats.Sizes.BucketsCount,
				BucketsList:      stats.Stats.Sizes.BucketsList,
				TotalTxCount:     stats.Stats.Sizes.TotalTxCount,
				TotalTxSizeBytes: stats.Stats.Sizes.TotalTxSizeBytes,
			},
			TxTagsStats: MempoolStatsTxTags{
				Enabled:              stats.Stats.Tags.Enabled,
				TotalTagsCount:       stats.Stats.Tags.TotalTagsCount,
				TotalMissedTagsCount: stats.Stats.Tags.TotalMissedTagsCount,
				BucketsCount:         stats.Stats.Tags.BucketsCount,
				BucketsList:          stats.Stats.Tags.BucketsList,
			},
		}

		return c.JSON(resp)
	}

	return c.SendStatus(fiber.StatusNotFound)
}

type AddressStats struct {
	Script              string `json:"script"`
	Type                string `json:"type"`
	Balance             int64  `json:"balance"`
	TotalConfirmedTx    int64  `json:"total_confirmed_tx"`
	TotalConfirmedUTXOs int64  `json:"total_confirmed_utxos"`
	SatoshisReceived    int64  `json:"satoshis_received"`
	SatoshisSpent       int64  `json:"satoshis_spent"`
}

func (s *Server) GetAddressStats(c *fiber.Ctx) error {

	address := c.Params("addressOrScripthash")

	if address == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}

	addressToScripts := getAddressScriptResponse(address)

	if len(addressToScripts) == 0 {
		return c.SendStatus(fiber.StatusNotFound)
	}

	addresses := make([]AddressStats, len(addressToScripts))

	for i, item := range addressToScripts {

		balance, err := utxostore.GetBalancePITB(item.Scripthash)

		if err != nil {
			break
		}

		addresses[i] = AddressStats{
			Script:              item.Scripthash,
			Type:                item.Type,
			Balance:             balance.Unspent.Satoshis,
			TotalConfirmedTx:    int64(balance.TotalTxsQty),
			TotalConfirmedUTXOs: int64(balance.Unspent.Qty),
			SatoshisReceived:    balance.Unspent.SatoshisReceived,
			SatoshisSpent:       balance.Unspent.SatoshisSpent,
		}
	}

	if len(addresses) == 0 {
		return c.SendStatus(fiber.StatusNotFound)
	}
	return c.JSON(addresses)
}

func (s *Server) GetScriptStats(c *fiber.Ctx) error {

	script := c.Params("addressOrScripthash")

	if script == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}

	balance, err := utxostore.GetBalancePITB(script)

	if err != nil {
		logger.Log.Error("GetScriptStats", zap.Error(err))
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	addressStats := AddressStats{
		Script:              script,
		Balance:             balance.Unspent.Satoshis,
		TotalConfirmedTx:    int64(balance.TotalTxsQty),
		TotalConfirmedUTXOs: int64(balance.Unspent.Qty),
		SatoshisReceived:    balance.Unspent.SatoshisReceived,
		SatoshisSpent:       balance.Unspent.SatoshisSpent,
	}

	return c.JSON(addressStats)
}
