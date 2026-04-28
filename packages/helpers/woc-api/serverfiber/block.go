package serverfiber

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/teranode-group/common"
	"github.com/teranode-group/common/logger"
	"github.com/teranode-group/proto/bstore"

	"github.com/gofiber/fiber/v2"
	woc_api_bstore "github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/internal"
	"github.com/teranode-group/woc-api/redis"
	"go.uber.org/zap"
)

type BlockHeadersList struct {
	Files []string `json:"files,omitempty"`
}

var LATEST_BLOCK_HEADERS_CACHE_KEY = "latest_block_headers"
var LATEST_BLOCK_HEADERS_HEIGHT_RANGE = "latest_block_headers_height_range"
var currentBlockHash string
var currentLastHeight int64

func extractBlockNumber(filename string) (int, error) {
	re := regexp.MustCompile(`(\d+)_(\d+)_headers\.bin`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) < 3 {
		return 0, fmt.Errorf("invalid filename format: %s", filename)
	}

	// Convert the first matched group (the first block number) to an integer
	blockNum := matches[1]
	return strconv.Atoi(blockNum)
}

func (s *Server) BlockHeadersFileResources(c *fiber.Ctx) error {

	var blockHeadersList = &BlockHeadersList{}
	var filePath = configs.Settings.BlockHeadersPath
	var url = configs.Settings.BlockHeadersFileUrl

	files, err := os.ReadDir(filePath)
	if err != nil {
		return fmt.Errorf("error reading directory %w", err)
	}

	var binFiles []string

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".bin") {
			binFiles = append(binFiles, file.Name())
		}
	}

	// Sort binFiles by extracting the block numbers
	sort.Slice(binFiles, func(i, j int) bool {
		blockNumI, _ := extractBlockNumber(binFiles[i])
		blockNumJ, _ := extractBlockNumber(binFiles[j])
		return blockNumI < blockNumJ
	})

	// Construct the URLs for the sorted files directly
	for _, filename := range binFiles {
		blockHeadersList.Files = append(blockHeadersList.Files, fmt.Sprintf("%s/%s", url, filename))
	}

	blockHeadersList.Files = append(blockHeadersList.Files, fmt.Sprintf("%s/latest", url))

	return c.JSON(blockHeadersList)
}

func (s *Server) BlockHeadersFile(c *fiber.Ctx) error {

	filePath := configs.Settings.BlockHeadersPath
	filename := c.Params("filename")
	fullPath := fmt.Sprintf("%s/%s", filePath, filename)

	_, err := os.Stat(fullPath)

	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("file not found")
	}

	return c.SendFile(fullPath)
}

func (s *Server) BlockHeadersLatest(c *fiber.Ctx) error {
	var (
		data            []byte
		height_ranges   string
		err             error
		count           uint64
		MAX_COUNT_VALUE uint64 = 100
		latestHeight    int64
	)

	countQuery := c.Query("count")

	if countQuery != "" {
		var err error
		count, err = strconv.ParseUint(countQuery, 10, 32)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid count value")
		}

		if count > MAX_COUNT_VALUE {
			return c.Status(fiber.StatusBadRequest).SendString("Count needs to be less then a 100")
		}
	}

	if count == 0 {
		if redis.RedisClient.Enabled {
			err = redis.GetCachedValue(LATEST_BLOCK_HEADERS_CACHE_KEY, &data, nil)
			if err != nil {
				logger.Log.Error("unable to to get cached latest block headers: %w", zap.Error(err))
				return c.Status(fiber.StatusInternalServerError).SendString("")
			}
			err = redis.GetCachedValue(LATEST_BLOCK_HEADERS_HEIGHT_RANGE, &height_ranges, nil)

			if err != nil {
				logger.Log.Error("latest_block_headers_height_range %w", zap.Error(err))
				return c.Status(fiber.StatusInternalServerError).SendString("")
			}
		}
	}

	if data == nil {
		data, latestHeight, err = s.GetLatestBlockHeaders(count)
		if err != nil {
			logger.Log.Error("failed getting latest block header from route call: %w", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).SendString("")
		}
	}

	if count > 0 {
		startHeight := latestHeight - int64(count)
		height_ranges = fmt.Sprintf("%d_%d", startHeight, latestHeight)
	}
	c.Set("Content-Disposition", "attachment; filename="+height_ranges+"_headers.bin")

	return c.Send(data)
}

func (s *Server) GetLatestBlockHeaders(count uint64) ([]byte, int64, error) {

	var filePath = configs.Settings.BlockHeadersPath
	var lastHeight int64

	chainInfo, err := bitcoinClient.GetBlockchainInfo()
	if err != nil {
		return nil, 0, fmt.Errorf("unable to GetBlockchainInfo: %w", err)
	}

	latestHeight := int64(chainInfo.Blocks)
	latestBlockHash := chainInfo.BestBlockHash

	if count == 0 {
		lastHeightFile := fmt.Sprintf("%s/last_height", filePath)

		lastHeightData, err := internal.ReadFromFile(lastHeightFile)
		if err != nil {
			return nil, 0, fmt.Errorf("error reading latest height file: %w", err)
		}

		lastHeight, err = strconv.ParseInt(strings.TrimSpace(string(lastHeightData)), 10, 32)
		if err != nil {
			return nil, 0, fmt.Errorf("error parsing last height %w", err)
		}
		//Increment by one as we want to go one ahead
		lastHeight++

		if lastHeight == currentLastHeight && latestBlockHash == currentBlockHash {
			logger.Log.Info("latest height and last stored height are up to date")
			return nil, 0, nil
		}

		currentLastHeight = lastHeight
		currentBlockHash = latestBlockHash
	} else {
		lastHeight = latestHeight - int64(count)
	}

	//TODO: NOTE: Keeping connection open for long time and reusing it
	wait, err := time.ParseDuration("10m")
	if err != nil {
		return nil, 0, fmt.Errorf("unable to parse time: %+v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	conn, err := common.GetGRPCConnection(ctx, "bstore")
	if err != nil {
		return nil, 0, fmt.Errorf("error: Unable to connect bstore: %+v", err)
	}
	defer conn.Close()

	bStoreClient := bstore.NewBStoreClient(conn)

	var blockheaders []byte

	for i := lastHeight; i < latestHeight+1; i++ {
		header, err := bStoreClient.GetBlockHeader(ctx, &bstore.GetBlockHeaderRequest{
			Height: uint64(i),
		})
		if err != nil {
			return nil, 0, fmt.Errorf("error: Unable to connect bstore: %+v", err)
		}

		versionBytes, err := hex.DecodeString(header.VersionHex)
		if err != nil {
			return nil, 0, fmt.Errorf("error: Unable to connect bstore: %+v", err)
		}

		Bits, err := hex.DecodeString(header.Bits)
		if err != nil {
			return nil, 0, fmt.Errorf("error: Unable to connect bstore: %+v", err)
		}

		rootBytes, err := hex.DecodeString(header.MerkleRoot)
		if err != nil {
			return nil, 0, fmt.Errorf("error: Unable to connect bstore: %+v", err)
		}

		time := make([]byte, 4)
		binary.LittleEndian.PutUint32(time, uint32(header.Time))

		nonce := make([]byte, 4)
		binary.LittleEndian.PutUint32(nonce, uint32(header.Nonce))

		previousBlockHash := header.PreviousBlockHash

		//Check if this is genesis block and set the previous block has as it won't exist
		if header.Hash == "000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f" ||
			header.Hash == "000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943" {
			previousBlockHash = "0000000000000000000000000000000000000000000000000000000000000000"
		}

		p, _ := hex.DecodeString(previousBlockHash)

		a := []byte{}
		a = append(a, reverseBytes(versionBytes)...)
		a = append(a, reverseBytes(p)...)
		a = append(a, reverseBytes(rootBytes)...)
		a = append(a, time...)
		a = append(a, reverseBytes(Bits)...)
		a = append(a, nonce...)

		blockheaders = append(blockheaders, a...)

	}

	if count == 0 {
		if redis.RedisClient.Enabled {
			conn := redis.RedisClient.ConnPool.Get()
			defer conn.Close()

			err = redis.SetCacheValue(LATEST_BLOCK_HEADERS_CACHE_KEY, blockheaders, conn)
			if err != nil {
				logger.Log.Error("unable to cache latest_block_headers %w", zap.Error(err))
			}

			range_value := fmt.Sprintf("%d_%d", lastHeight, latestHeight)

			err = redis.SetCacheValue(LATEST_BLOCK_HEADERS_HEIGHT_RANGE, range_value, conn)
			if err != nil {
				logger.Log.Error("latest_block_headers_height_range %w", zap.Error(err))
			}
		}

	}

	return blockheaders, latestHeight, nil

}

func (s *Server) RawTransactionByBlockHeightAndTxIndex(c *fiber.Ctx) error {
	println("Entering RawTransactionByBlockHeightAndTxIndex")

	heightParam := c.Params("height")
	txindexParam := c.Params("txindex")

	// Parse height.
	height, err := strconv.Atoi(heightParam)
	if err != nil || height < 0 {
		return fmt.Errorf("failed to read fields: invalid height parameter %q: %w", heightParam, err)
	}

	txindex, err := strconv.Atoi(txindexParam)
	if err != nil || txindex < 0 {
		return fmt.Errorf("failed to read fields: invalid txindex parameter %q: %w", txindexParam, err)
	}

	var txid string

	// Try using bstore lookup.
	tx, err := woc_api_bstore.GetTransactionByBlockHeightAndIndex(int64(height), txindex, 10)
	if err == nil && tx != nil {
		txid = tx.TxID
	} else {
		// Fallback: use bitcoinClient.
		hash, err := bitcoinClient.GetBlockHash(height)
		if err != nil {
			if strings.Contains(err.Error(), "out of range") {
				return c.SendStatus(fiber.StatusNotFound)
			} else {
				return fmt.Errorf("failed to get block hash for height %d: %w", height, err)
			}
		}
		resp, err := bitcoinClient.GetBlock(hash)
		if err != nil {
			return fmt.Errorf("failed to get block for hash %s: %w", hash, err)
		}
		if txindex >= len(resp.Tx) {
			logger.Log.Error("transaction index %d out of range: block has %d transactions", zap.Int("txindex", txindex), zap.Int("No of tx", len(resp.Tx)))
			return c.SendStatus(fiber.StatusNotFound)
		}
		txid = resp.Tx[txindex]
	}

	rawTx, err := internal.GetTransaction(txid)
	if err != nil {
		return fmt.Errorf("failed to get transaction for txid %q: %w", txid, err)
	}

	if rawTx == nil {
		return c.SendStatus(fiber.StatusNotFound)
	}

	// Patch for block 0 Satoshi tx if needed.
	if txid == "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b" {
		txPatch, err := internal.GetTransaction("0e3e2357e806b6cdb1f70b54c3a3a17b6714ee1f0e68bebb44a74b1efd512098")
		if err != nil {
			return fmt.Errorf("failed to patch transaction for txid %q: %w", txid, err)
		}
		rawTx.Confirmations = txPatch.Confirmations + 1
	}
	return c.JSON(rawTx)
}

func reverseBytes(a []byte) []byte {
	tmp := make([]byte, len(a))
	copy(tmp, a)

	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}
	return tmp
}
