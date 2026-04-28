package mongocache

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	bitcoin "github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	// "github.com/mongodb/mongo-go-driver/bson"
	// "github.com/mongodb/mongo-go-driver/mongo"
	// "github.com/mongodb/mongo-go-driver/mongo/options"
)

var logger = gocore.Log("woc-api")

const mongoBlockCollection string = "blocks"
const blockTxidCollection string = "blocktxids"
const transactionCollection string = "transactions"
const BlockTxidCollectionMaxTx int = 50000
const MaxTxids int = 100 // the maximum number of tx we will store with the block

var cacheMutex = &sync.RWMutex{}

var readClient *mongo.Client
var writeClient *mongo.Client
var healthCheckClient *mongo.Client
var client *mongo.Client // used by other stuff

func MongoHealthCheck() (ok bool) {

	//do it once
	if healthCheckClient == nil {

		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Failed creating mongo connection url %+v\n", err)
			return false
		}

		healthCheckClient, err = getNewMongoClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Failed creating mongo client %+v\n", err)
			return false
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = healthCheckClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Failed connecting to mongo %+v\n", err)
			return false

		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := healthCheckClient.Ping(ctx, nil)
	if err != nil {
		logger.Errorf("Failed to ping mongo %+v\n", err)
		return false
	}

	return true
}

// GetBlockFromCache comment
func GetBlockFromCache(hash string) (block *bitcoin.Block, ok bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	if readClient == nil {
		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Error creating mongo connection url %+v\n", err)
			return
		}

		readClient, err = getNewMongoClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = readClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Error connecting to mongo %+v\n", err)
			return
		}
		// defer client.Disconnect(ctx)
	}

	collection := readClient.Database(getMongoDBName()).Collection(mongoBlockCollection)

	block = &bitcoin.Block{}
	filter := bson.M{"hash": hash}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, filter).Decode(&block)
	if err != nil {
		return
	}

	ok = true
	return
}

// AddBlockToCache comment
func AddBlockToCache(block bitcoin.Block) {
	logger.Info("AddBlockToCache for block\n")
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if writeClient == nil {
		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Error creating mongo connection url %+v\n", err)
			return
		}
		writeClient, err = getNewMongoClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = writeClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Error connecting to mongo %+v\n", err)
			return
		}
		// defer client.Disconnect(ctx)
	}

	txCount := len(block.Tx)

	// if number of tx is > maxTxids cache block and put txids in separate collection
	if txCount > MaxTxids {
		txidsToSaveInBlock := block.Tx[:MaxTxids]
		excessTxids := block.Tx[MaxTxids:]
		block.Tx = txidsToSaveInBlock

		evenSizedDocs := len(excessTxids) / BlockTxidCollectionMaxTx
		extraDoc := len(excessTxids) % BlockTxidCollectionMaxTx

		btxids := make([]interface{}, evenSizedDocs+1)
		documentsTosave := evenSizedDocs
		if evenSizedDocs > 0 {
			for i := 0; i < evenSizedDocs; i++ {
				//fmt.Println(i*50000, (i*50000)+50000)
				startIndex := uint64(i*BlockTxidCollectionMaxTx) + uint64(MaxTxids)
				count := uint64(BlockTxidCollectionMaxTx)
				endIndex := startIndex + (count - 1)

				btxids[i] = &bitcoin.BlockTxid{BlockHash: block.Hash, Tx: excessTxids[i*BlockTxidCollectionMaxTx : (i*BlockTxidCollectionMaxTx)+BlockTxidCollectionMaxTx], StartIndex: startIndex, EndIndex: endIndex, Count: count}
			}
			if extraDoc > 0 {
				//fmt.Println(evenSizedDocs*50000, evenSizedDocs*50000+extraDoc)
				startIndex := uint64(evenSizedDocs*BlockTxidCollectionMaxTx) + uint64(MaxTxids)
				count := uint64(extraDoc)
				endIndex := startIndex + (count - 1)

				btxids[evenSizedDocs] = &bitcoin.BlockTxid{BlockHash: block.Hash, Tx: excessTxids[evenSizedDocs*BlockTxidCollectionMaxTx:], StartIndex: startIndex, EndIndex: endIndex, Count: count}
				documentsTosave++
			}
		} else {
			if extraDoc > 0 {
				documentsTosave++
				startIndex := uint64(evenSizedDocs*BlockTxidCollectionMaxTx) + uint64(MaxTxids)
				count := uint64(extraDoc)
				endIndex := startIndex + (count - 1)

				btxids[evenSizedDocs] = &bitcoin.BlockTxid{BlockHash: block.Hash, Tx: excessTxids, StartIndex: startIndex, EndIndex: endIndex, Count: count}
			}
		}

		collection := writeClient.Database(getMongoDBName()).Collection(blockTxidCollection)
		//btxids := make([]interface{}, len(excessTxids))
		// for index, txid := range btxids {
		// 	if(index < documentsTosave )
		// 	btxids[index] = &models.BlockTxid{BlockHash: block.Hash, Txid: txid}
		// }
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := collection.InsertMany(ctx, btxids[:documentsTosave])
		if err != nil {
			logger.Errorf("inserting many btxids %+v\n", err)
		}
	}

	collection := writeClient.Database(getMongoDBName()).Collection(mongoBlockCollection)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := collection.InsertOne(ctx, block)
	if err != nil {
		logger.Errorf("can't insert block into mongo collection %+v\n", err)
		return
	}

	logger.Infof("Inserted block into mongo. Id:  %+v\n", result.InsertedID)
}

// DeleteBlockFromCacheByHeight comment
func DeleteBlockFromCacheByHeight(height uint64) (ok bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	if writeClient == nil {
		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Error creating mongo connection url %+v\n", err)
			return
		}

		writeClient, err = getNewMongoClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = writeClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Error connecting to mongo %+v\n", err)
			return
		}
		// defer client.Disconnect(ctx)
	}
	collection := writeClient.Database(getMongoDBName()).Collection(mongoBlockCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	filter := bson.M{"height": height}
	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return
	}

	ok = true
	return
}

// GetTxidsForBlock comment
func GetTxidsForBlock(hash string, skip int, limit int) (txids []string, err error) {

	if readClient == nil {
		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Error creating mongo connection url %+v\n", err)
			return nil, err
		}

		readClient, err = getNewMongoClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = readClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Error connecting to mongo %+v\n", err)
			return nil, err
		}
		// defer readClient.Disconnect(ctx)
	}
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	collection := readClient.Database(getMongoDBName()).Collection(blockTxidCollection)
	// options := options.FindOptions{}
	//TODO: This doesn't look right
	if skip > BlockTxidCollectionMaxTx {
		skip = skip - (skip/BlockTxidCollectionMaxTx)*BlockTxidCollectionMaxTx
	}

	//TODO: This doesn't look right
	projection := bson.M{"tx": bson.M{"$slice": []int{skip - MaxTxids, limit}}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	filter := bson.M{"blockhash": hash, "startindex": bson.M{"$lte": skip}, "endindex": bson.M{"$gte": skip}}

	cur, err := collection.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		logger.Errorf("can't query mongo collection %+v\n", err)
		return
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var txResult TxResult
		err = cur.Decode(&txResult)
		if err != nil {
			logger.Errorf("Error decoding txids %+v", err)
			return
		}

		txids = append(txids, txResult.Tx...)
	}
	if err := cur.Err(); err != nil {
		logger.Errorf("GetTxidsForBlock: %+v", err)
	}
	return
}

// GetTxidsForAPIBlockPage comment
func GetTxidsForAPIBlockPage(hash string, skip int, limit int) (txids []string, err error) {

	if readClient == nil {
		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Error creating mongo connection url %+v\n", err)
			return nil, err
		}

		readClient, err = getNewMongoClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = readClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Error connecting to mongo %+v\n", err)
			return nil, err
		}
		// defer readClient.Disconnect(ctx)
	}
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	collection := readClient.Database(getMongoDBName()).Collection(blockTxidCollection)
	//options := options.FindOptions{}
	// if skip > blockTxidCollectionMaxTx {
	// 	skip = skip - (skip/blockTxidCollectionMaxTx)*blockTxidCollectionMaxTx
	// }

	projection := bson.M{"tx": bson.M{"$slice": []int{0, limit}}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	filter := bson.M{"blockhash": hash, "startindex": bson.M{"$lte": skip}, "endindex": bson.M{"$gte": skip}}

	cur, err := collection.Find(ctx, filter, options.Find().SetProjection(projection))
	if err != nil {
		logger.Errorf("can't query mongo collection %+v\n", err)
		return
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var txResult TxResult
		err = cur.Decode(&txResult)
		if err != nil {
			logger.Errorf("Error decoding txids %+v", err)
			return
		}

		txids = append(txids, txResult.Tx...)
	}
	if err := cur.Err(); err != nil {
		logger.Errorf("GetTxidsForAPIBlockPage %+v", err)
	}
	return
}

// TxResult comment
type TxResult struct {
	Tx []string `json:"tx"`
}

// AddTransactionToCache comment
func AddTransactionToCache(tx bitcoin.RawTransaction) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if writeClient == nil {
		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Error creating mongo connection url %+v\n", err)
			return
		}
		writeClient, err = getNewMongoClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = writeClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Error connecting to mongo %+v\n", err)
			return
		}
		//defer client.Disconnect(ctx)
	}
	collection := writeClient.Database(getMongoDBName()).Collection(transactionCollection)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := collection.InsertOne(ctx, tx)
	if err != nil {
		// logger.Errorf("Error: can't insert transaction into mongo collection %+v\n", err)
		return
	}

}

// GetTransactionFromCache comment
func GetTransactionFromCache(txid string) (tx *bitcoin.RawTransaction, ok bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()

	if readClient == nil {
		mongoConnectionURL, err := getConnectionURL()
		if err != nil {
			logger.Errorf("Error creating mongo connection url %+v\n", err)
			return
		}

		readClient, err = getNewMongoClient(mongoConnectionURL)

		// client, err := mongo.NewClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = readClient.Connect(ctx)
		if err != nil {
			logger.Errorf("Error connecting to mongo %+v\n", err)
			return
		}
		//defer client.Disconnect(ctx)
	}
	collection := readClient.Database(getMongoDBName()).Collection(transactionCollection)
	filter := bson.M{"txid": txid}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := collection.FindOne(ctx, filter).Decode(&tx)
	if err != nil {
		// logger.Errorf("Error: can't query mongo collection %+v\n", err)
		return
	}

	ok = true
	// truncate long txs
	tx.Hex = ""

	maxHex := getMaxTxHexLength()
	for i, o := range tx.Vout {
		if len(o.ScriptPubKey.Hex) > maxHex {
			tx.Vout[i].ScriptPubKey.Hex = truncateStrings(o.ScriptPubKey.Hex, maxHex)
			tx.Vout[i].ScriptPubKey.ASM = truncateStrings(o.ScriptPubKey.ASM, maxHex)
		}
	}
	return
}

func getMongoDBName() (dbName string) {
	dbName, _ = gocore.Config().Get("mongoDatabase")
	return
}

func getMaxTxHexLength() (maxTxHexLength int) {
	maxTxHexLength, _ = gocore.Config().GetInt("maxTxHexLength")
	return
}

func getConnectionURL() (mongoConnectionURL string, err error) {
	mongoHost, ok := gocore.Config().Get("mongoHost")
	if !ok {
		logger.Fatal("Must have a mongo host setting")
	}
	mongoPort, ok := gocore.Config().GetInt("mongoPort")
	if !ok {
		logger.Fatal("Must have a mongo port setting")
	}
	mongoUsername, ok := gocore.Config().Get("mongoUsername")
	if !ok {
		logger.Fatal("Must have a mongo username setting")
	}
	mongoPassword, ok := gocore.Config().Get("mongoPassword")
	if !ok {
		logger.Fatal("Must have a mongo password setting")
	}
	mongoDatabase, ok := gocore.Config().Get("mongoDatabase")
	if !ok {
		logger.Fatal("Must have a mongo database setting")
	}

	mongoConnectionURL = fmt.Sprintf(`mongodb://%s:%s@%s:%d/%s`,
		mongoUsername,
		mongoPassword,
		mongoHost,
		mongoPort,
		mongoDatabase,
	)
	return
}

func getMongoClient(mongoConnectionURL string) (client *mongo.Client, err error) {
	if client == nil {
		client, err = mongo.NewClient(options.Client().ApplyURI(mongoConnectionURL))

		// client, err = mongo.NewClient(mongoConnectionURL)
		if err != nil {
			logger.Errorf("Error creating mongo client %+v\n", err)
			return
		}
	}
	return client, nil
}

func getNewMongoClient(mongoConnectionURL string) (client *mongo.Client, err error) {
	return mongo.NewClient(options.Client().ApplyURI(mongoConnectionURL))
}

func truncateStrings(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for !utf8.ValidString(s[:n]) {
		n--
	}
	return s[:n]
}
