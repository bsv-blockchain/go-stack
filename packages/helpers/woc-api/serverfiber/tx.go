package serverfiber

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/gofiber/fiber/v2"

	"github.com/ordishs/go-bitcoin"
	"github.com/teranode-group/common/bsdecoder"
	"github.com/teranode-group/common/logger"
	"go.uber.org/zap"
	bstore_proto "github.com/teranode-group/proto/bstore"
	p2p_service "github.com/teranode-group/proto/p2p-service"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/internal"
)

type TxPropogation struct {
	QueriedPeers int64 `json:"queried_peers"`
	FoundOnPeers int64 `json:"found_on_peers"`
}

func (s *Server) TxPropagation(c *fiber.Ctx) error {
	txid := c.Params("txid")
	var noOfPeersPropagatedTo int64

	if len(txid) == 64 {
		var err error
		txPropogationResponse, err := s.p2pServiceClient.GetTxsPropagation(c.UserContext(), &p2p_service.GetRawTxsFromNodeRequest{
			Txids: []string{txid},
		})
		if err != nil {
			return fmt.Errorf("failed to get tx propagation: %w", err)
		}

		if len(txPropogationResponse.PropagatedTxs) > 0 {
			noOfPeersPropagatedTo = int64(len(txPropogationResponse.PropagatedTxs[0].Peers))
		}

		txPropogationJson := &TxPropogation{
			QueriedPeers: txPropogationResponse.ConnectedPeers,
			FoundOnPeers: noOfPeersPropagatedTo,
		}
		return c.JSON(txPropogationJson)
	} else {
		return c.Status(fiber.StatusInternalServerError).SendString("Invalid txid")
	}
}

const (
	maxFetchRate     = 300               // max fetches per second
	maxFetchDuration = 10 * time.Second  // max wall-clock time for BEEF resolution
)

// I/O hooks — package-level so tests can swap them.
var (
	fetchRawHex  = getRawTxHex
	resolveProof = defaultResolveProof
)

// defaultResolveProof wraps ParseRawTx → fetchProof → convertProof.
func defaultResolveProof(rawHex string) (*transaction.MerklePath, error) {
	bsTx, err := ParseRawTx(rawHex)
	if err != nil {
		return nil, err
	}
	proof, _ := fetchProof(bsTx)
	if proof == nil || len(proof.Nodes) == 0 {
		return nil, nil
	}
	return convertProof(proof)
}

func (s *Server) Bump(c *fiber.Ctx) error {
	txid := c.Params("txid")
	if len(txid) != 64 {
		return c.
			Status(fiber.StatusBadRequest).
			SendString("txid must be 64 hex characters")
	}

	rawHex, err := fetchRawHex(txid)
	if err != nil {
		logger.Log.Error("bump: fetch raw hex", zap.String("txid", txid), zap.Error(err))
		return c.
			Status(fiber.StatusInternalServerError).
			SendString(fmt.Sprintf("failed to fetch raw tx: %v", err))
	}

	mp, err := resolveProof(rawHex)
	if err != nil {
		logger.Log.Error("bump: convert proof", zap.String("txid", txid), zap.Error(err))
		return c.
			Status(fiber.StatusInternalServerError).
			SendString(fmt.Sprintf("failed to convert proof: %v", err))
	}
	if mp == nil {
		return c.
			Status(fiber.StatusNotFound).
			SendString("no merkle proof available for this transaction")
	}

	c.Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.bump.hex", txid))
	return c.SendString(mp.Hex())
}

func (s *Server) Beef(c *fiber.Ctx) error {
	txid := c.Params("txid")
	if len(txid) != 64 {
		return c.
			Status(fiber.StatusBadRequest).
			SendString("txid must be 64 hex characters")
	}

	rawHex, err := getRawTxHex(txid)
	if err != nil {
		logger.Log.Error("beef: fetch raw hex", zap.String("txid", txid), zap.Error(err))
		return c.
			Status(fiber.StatusInternalServerError).
			SendString(fmt.Sprintf("failed to fetch raw tx: %v", err))
	}

	rawBytes, _ := hex.DecodeString(rawHex)
	sdkTx := &transaction.Transaction{}
	if _, err := sdkTx.ReadFrom(bytes.NewReader(rawBytes)); err != nil {
		logger.Log.Error("beef: parse tx", zap.String("txid", txid), zap.Error(err))
		return c.
			Status(fiber.StatusInternalServerError).
			SendString(fmt.Sprintf("failed to parse raw tx: %v", err))
	}

	// 3) if confirmed, attach proof and emit single-tx BEEF
	bsTx, _ := ParseRawTx(rawHex)
	proof, _ := fetchProof(bsTx)
	if proof != nil && len(proof.Nodes) > 0 {
		mp, err := convertProof(proof)
		if err == nil {
			_ = sdkTx.AddMerkleProof(mp)
		}
		beefBytes, err := sdkTx.BEEF()
		if err != nil {
			logger.Log.Error("beef: BEEF()", zap.String("txid", txid), zap.Error(err))
			return c.
				Status(fiber.StatusInternalServerError).
				SendString(fmt.Sprintf("failed to build BEEF: %v", err))
		}
		c.Set("Content-Disposition",
			fmt.Sprintf("attachment; filename=%s.beef.hex", txid))
		return c.SendString(hex.EncodeToString(beefBytes))
	}

	// 4) reject unconfirmed transactions with OP_RETURN outputs — these are
	//    data carriers that can form very long unconfirmed chains.
	for _, out := range sdkTx.Outputs {
		if out.LockingScript != nil && out.LockingScript.IsData() {
			return c.
				Status(fiber.StatusUnprocessableEntity).
				SendString("unconfirmed transactions with data (OP_RETURN) outputs are not supported by the BEEF endpoint")
		}
	}

	// 5) unconfirmed: recurse parents with time-bounded context
	ctx, cancel := context.WithTimeout(c.UserContext(), maxFetchDuration)
	defer cancel()
	cache := make(map[string]*transaction.Transaction)
	rootTree, err := s.buildSdkBeef(ctx, txid, cache)
	if err != nil {
		logger.Log.Error("beef: buildSdkBeef", zap.String("txid", txid), zap.Error(err))
		return c.
			Status(fiber.StatusInternalServerError).
			SendString(fmt.Sprintf("cannot build BEEF: %v", err))
	}

	// 6) emit partial BEEF
	beefBytes, err := rootTree.BEEF()
	if err != nil {
		logger.Log.Error("beef: partial BEEF()", zap.String("txid", txid), zap.Error(err))
		return c.
			Status(fiber.StatusInternalServerError).
			SendString(fmt.Sprintf("failed to build partial BEEF: %v", err))
	}

	c.Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.beef.hex", txid))
	return c.SendString(hex.EncodeToString(beefBytes))
}

func (s *Server) buildSdkBeef(
	ctx context.Context,
	txid string,
	cache map[string]*transaction.Transaction,
) (*transaction.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("cannot build BEEF: %w", err)
	}
	if t, ok := cache[txid]; ok {
		return t, nil
	}

	// Rate-limit: sleep to stay within maxFetchRate.
	// time.Sleep(time.Second / maxFetchRate) ≈ 3.3ms per fetch.
	sleepDur := time.Second / time.Duration(maxFetchRate)
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("cannot build BEEF: %w", ctx.Err())
	case <-time.After(sleepDur):
	}

	rawHex, err := fetchRawHex(txid)
	if err != nil {
		return nil, err
	}
	rawBytes, _ := hex.DecodeString(rawHex)
	tx := &transaction.Transaction{}
	if _, err := tx.ReadFrom(bytes.NewReader(rawBytes)); err != nil {
		return nil, err
	}
	cache[txid] = tx

	if mp, err := resolveProof(rawHex); err == nil && mp != nil {
		if err := tx.AddMerkleProof(mp); err != nil {
			logger.Log.Error("beef: AddMerkleProof", zap.String("txid", txid), zap.Error(err))
		}
		return tx, nil
	}

	// unconfirmed: recurse into every input
	for i := range tx.Inputs {
		childID := tx.Inputs[i].SourceTXID.String()
		child, err := s.buildSdkBeef(ctx, childID, cache)
		if err != nil {
			return nil, err
		}
		if child != nil {
			tx.Inputs[i].SourceTransaction = child
		}
	}
	return tx, nil
}

func reverseBytesForProof(b []byte) {
	for i := 0; i < len(b)/2; i++ {
		b[i], b[len(b)-1-i] = b[len(b)-1-i], b[i]
	}
}

// convertProof turns a WOC/RPC MerkleProof into the SDK's MerklePath,
// reversing each hash to little-endian before building the PathElements.
func convertProof(p *bitcoin.MerkleProof) (*transaction.MerklePath, error) {
	// 1) grab the header so we know the block height
	hdr, err := internal.GetBlockHeader(p.Target)
	if err != nil {
		return nil, err
	}

	// 2) allocate the MerklePath
	mp := &transaction.MerklePath{
		BlockHeight: uint32(hdr.Height),
		Path:        make([][]*transaction.PathElement, len(p.Nodes)),
	}

	// 3) decode & reverse your leaf txid
	leafRaw, _ := hex.DecodeString(p.TxOrId)
	reverseBytesForProof(leafRaw)
	leafHash, _ := chainhash.NewHash(leafRaw)

	// we’ll need a &true for the Txid flag
	isLeaf := true
	leafElem := &transaction.PathElement{
		Hash:   leafHash,
		Offset: uint64(p.Index),
		Txid:   &isLeaf,
	}

	// 4) for each sibling node, decode, reverse and build PathElement
	for i, h := range p.Nodes {
		offset := uint64((p.Index >> uint(i)) ^ 1)

		var sibling *transaction.PathElement
		if h == "*" {
			dup := true
			sibling = &transaction.PathElement{
				Offset:    offset,
				Duplicate: &dup,
			}
		} else {
			nodeRaw, _ := hex.DecodeString(h)
			reverseBytesForProof(nodeRaw)
			nodeHash, _ := chainhash.NewHash(nodeRaw)
			sibling = &transaction.PathElement{
				Hash:   nodeHash,
				Offset: offset,
			}
		}

		if i == 0 {
			// layer zero carries both leaf & sibling, in the correct order
			if int(p.Index)%2 == 0 {
				mp.Path[0] = []*transaction.PathElement{sibling, leafElem}
			} else {
				mp.Path[0] = []*transaction.PathElement{leafElem, sibling}
			}
		} else {
			// deeper layers only need the one sibling
			mp.Path[i] = []*transaction.PathElement{sibling}
		}
	}

	return mp, nil
}

func fetchProof(tx *bsdecoder.RawTransaction) (*bitcoin.MerkleProof, error) {
	if configs.Settings.WocMerkleServiceEnabled {
		url := fmt.Sprintf("%s/proofs/%s", configs.Settings.WocMerkleServiceAddress, tx.TxID)
		if resp, err := http.Get(url); err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if len(body) > 0 {
				var arr []bitcoin.MerkleProof
				if json.Unmarshal(body, &arr) == nil && len(arr) > 0 {
					return &arr[0], nil
				}
			}
		}
	}
	proof, err := bitcoinClient.GetMerkleProof(tx.BlockHash, tx.TxID)
	if err != nil {
		/*if strings.Contains(err.Error(), "not found") {
			return &bitcoin.MerkleProof{Index: 1, TxOrId: tx.TxID, Target: tx.BlockHash}, nil
		}*/
		return nil, nil
	}
	return proof, nil
}

func getRawTxHex(txid string) (string, error) {
	if bstore.IsEnabled() {
		if hx, err := bstore.GetTxHex(txid); err == nil && hx != nil {
			return *hx, nil
		}
	}
	ptr, err := bitcoinClient.GetRawTransactionHex(txid)
	if err != nil {
		return "", err
	}
	if ptr != nil {
		return *ptr, nil
	}
	return "", fmt.Errorf("raw tx for %s not found", txid)
}

func ParseRawTx(raw string) (*bsdecoder.RawTransaction, error) {
	rawHex, _ := hex.DecodeString(raw)
	req := &bstore_proto.GetTransactionResponse{Raw: rawHex}
	return bsdecoder.DecodeRawTransaction(req, configs.Settings.IsMainnet)
}
