package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/teranode-group/common/utils"
)

// legacyProxy forwards a request to the Fiber server and reshapes the response.
func legacyProxy(pathBuilder func(map[string]string) string, reshape func([]byte) ([]byte, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		fiberURL := "http://localhost:8085" + pathBuilder(vars)

		var resp *http.Response
		var err error

		if r.Method == http.MethodPost {
			resp, err = http.Post(fiberURL, "application/json", r.Body)
		} else {
			resp, err = http.Get(fiberURL)
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if resp.StatusCode != http.StatusOK {
			w.WriteHeader(resp.StatusCode)
			w.Write(body)
			return
		}

		out, err := reshape(body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(out)
	}
}

// Reshape functions

// legacyHistory extracts "result" and reverses to ascending order
func legacyHistory(body []byte) ([]byte, error) {
	var resp struct {
		Result []json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return []byte("[]"), nil
	}
	for i, j := 0, len(resp.Result)-1; i < j; i, j = i+1, j-1 {
		resp.Result[i], resp.Result[j] = resp.Result[j], resp.Result[i]
	}
	return json.Marshal(resp.Result)
}

// legacyBulkHistory reshapes and reverses history to ascending order
// Handles both {result:[...]} and {confirmed:{result:[...]}, unconfirmed:{result:[...]}} shapes
func legacyBulkHistory(body []byte) ([]byte, error) {
	// Try /history/all shape first: {confirmed: {result: [...]}, unconfirmed: {result: [...]}}
	var allItems []struct {
		Address   string `json:"address"`
		Confirmed struct {
			Result []json.RawMessage `json:"result"`
		} `json:"confirmed"`
		Unconfirmed struct {
			Result []json.RawMessage `json:"result"`
		} `json:"unconfirmed"`
	}
	if err := json.Unmarshal(body, &allItems); err == nil && len(allItems) > 0 && allItems[0].Confirmed.Result != nil {
		type entry struct {
			Address string            `json:"address"`
			History []json.RawMessage `json:"history"`
			Error   string            `json:"error"`
		}
		result := make([]entry, len(allItems))
		for i, item := range allItems {
			// Combine confirmed + unconfirmed
			history := append(item.Confirmed.Result, item.Unconfirmed.Result...)
			if history == nil {
				history = []json.RawMessage{}
			}
			fixHistoryOrder(history)
			result[i] = entry{Address: item.Address, History: history, Error: ""}
		}
		return json.Marshal(result)
	}

	// Fallback: flat {result:[...]} shape
	var items []struct {
		Address string            `json:"address"`
		Result  []json.RawMessage `json:"result"`
		Error   string            `json:"error"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	type entry struct {
		Address string            `json:"address"`
		History []json.RawMessage `json:"history"`
		Error   string            `json:"error"`
	}
	result := make([]entry, len(items))
	for i, item := range items {
		history := item.Result
		if history == nil {
			history = []json.RawMessage{}
		}
		// Reverse to ascending
		for a, b := 0, len(history)-1; a < b; a, b = a+1, b-1 {
			history[a], history[b] = history[b], history[a]
		}
		result[i] = entry{Address: item.Address, History: history, Error: item.Error}
	}
	return json.Marshal(result)
}

// fixHistoryOrder reverses items within each height group to restore block_tx_index ASC order
func fixHistoryOrder(items []json.RawMessage) {
	if len(items) == 0 {
		return
	}
	type heightOnly struct {
		Height int32 `json:"height"`
	}
	getHeight := func(raw json.RawMessage) int32 {
		var h heightOnly
		json.Unmarshal(raw, &h)
		return h.Height
	}
	start := 0
	for i := 1; i <= len(items); i++ {
		if i == len(items) || getHeight(items[i]) != getHeight(items[start]) {
			for a, b := start, i-1; a < b; a, b = a+1, b-1 {
				items[a], items[b] = items[b], items[a]
			}
			start = i
		}
	}
}

// legacyUnspent extracts "result", strips to only {height, tx_pos, tx_hash, value}, sorts ascending
func legacyUnspent(body []byte) ([]byte, error) {
	var resp struct {
		Result []unspentItem `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return []byte("[]"), nil
	}
	fixUnspentOrder(resp.Result)
	return json.Marshal(resp.Result)
}

type unspentItem struct {
	Height uint32 `json:"height"`
	TxPos  uint32 `json:"tx_pos"`
	TxHash string `json:"tx_hash"`
	Value  int64  `json:"value"`
}

// fixUnspentOrder fixes the order of unspent items to match electrumX ordering.
// Data arrives from /unspent/all already in height ASC order (reversed by Fiber handler),
// but items within each height group have reversed block_tx_index order.
// This function reverses items within each height group to restore block_tx_index ASC.
func fixUnspentOrder(items []unspentItem) {
	if len(items) == 0 {
		return
	}
	start := 0
	for i := 1; i <= len(items); i++ {
		if i == len(items) || items[i].Height != items[start].Height {
			// Reverse this height group
			for a, b := start, i-1; a < b; a, b = a+1, b-1 {
				items[a], items[b] = items[b], items[a]
			}
			start = i
		}
	}
}

// legacyBulkUnspent reshapes, strips to only {height, tx_pos, tx_hash, value}, sorts ascending
func legacyBulkUnspent(body []byte) ([]byte, error) {
	var items []struct {
		Address string        `json:"address"`
		Result  []unspentItem `json:"result"`
		Error   string        `json:"error"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	type entry struct {
		Address string        `json:"address"`
		Unspent []unspentItem `json:"unspent"`
		Error   string        `json:"error"`
	}
	result := make([]entry, len(items))
	for i, item := range items {
		unspent := item.Result
		if unspent == nil {
			unspent = []unspentItem{}
		}
		fixUnspentOrder(unspent)
		result[i] = entry{Address: item.Address, Unspent: unspent, Error: item.Error}
	}
	return json.Marshal(result)
}

// legacyBalance reshapes {confirmed:N} → {confirmed:N, unconfirmed:0}
func legacyBalance(body []byte) ([]byte, error) {
	var resp struct {
		Confirmed int64 `json:"confirmed"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]int64{"confirmed": resp.Confirmed, "unconfirmed": 0})
}


// legacyBulkBalance reshapes [{address, confirmed:N}] → [{address, balance:{confirmed,unconfirmed}, error:""}]
func legacyBulkBalance(body []byte) ([]byte, error) {
	var items []struct {
		Address   string `json:"address"`
		Confirmed int64  `json:"confirmed"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	type balanceObj struct {
		Confirmed   int64 `json:"confirmed"`
		Unconfirmed int64 `json:"unconfirmed"`
	}
	type entry struct {
		Address string      `json:"address"`
		Balance *balanceObj `json:"balance"`
		Error   string      `json:"error"`
	}
	result := make([]entry, len(items))
	for i, item := range items {
		result[i] = entry{Address: item.Address, Balance: &balanceObj{Confirmed: item.Confirmed}, Error: item.Error}
	}
	return json.Marshal(result)
}


// Path builders

func addressToScriptHashPath(basePath string) func(map[string]string) string {
	return func(vars map[string]string) string {
		address := vars["address"]
		scriptHash, err := utils.AddressToScriptHash(address, network)
		if err != nil {
			return basePath
		}
		return strings.Replace(basePath, "{sh}", scriptHash, 1)
	}
}

// addressPassthroughPath forwards the address as-is so the downstream handler
// can do its own conversion AND populate the `address` variable, which enables
// the associated-scripthash merge in GetAddressConfirmed{Balance,History,Unspent}.
func addressPassthroughPath(basePath string) func(map[string]string) string {
	return func(vars map[string]string) string {
		return strings.Replace(basePath, "{addr}", vars["address"], 1)
	}
}

func scriptHashPath(basePath string) func(map[string]string) string {
	return func(vars map[string]string) string {
		return strings.Replace(basePath, "{sh}", vars["scriptHash"], 1)
	}
}

func staticPath(path string) func(map[string]string) string {
	return func(_ map[string]string) string { return path }
}
