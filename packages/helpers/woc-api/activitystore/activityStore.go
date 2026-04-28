package activitystore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/ordishs/gocore"
	"github.com/teranode-group/common"
	account_manager "github.com/teranode-group/proto/account-manager"
	"google.golang.org/grpc"
)

var (
	once       sync.Once
	maxItems   int
	maxTimeout time.Duration
	ch         chan *account_manager.AddActivityRequest
	mu         sync.Mutex
	conn       *grpc.ClientConn
	connOnce   sync.Once
	connErr    error
	logger     = gocore.Log("woc-api")
)

func getConn(ctx context.Context) (*grpc.ClientConn, error) {
	connOnce.Do(func() {
		m := make(map[string]string)
		m["is_admin"] = "true"

		conn, connErr = common.GetGRPCConnection(
			ctx,
			"accountManager",
			m,
		)
	})

	return conn, connErr
}

func resetConn() {
	mu.Lock()
	defer mu.Unlock()

	conn = nil
}

func AddActivity(req *account_manager.AddActivityRequest) {
	once.Do(func() {
		maxItems, _ = gocore.Config().GetInt("activityStore_maxItems", 1000)

		maxTimeoutStr, _ := gocore.Config().Get("activityStore_maxTimeout", "2s")

		var err error

		maxTimeout, err = time.ParseDuration(maxTimeoutStr)
		if err != nil {
			maxTimeout = 2 * time.Second
		}

		ch = make(chan *account_manager.AddActivityRequest)
		startWorker()
	})

	ch <- req
}

func startWorker() {
	// Start worker
	go func() {
		for {
			var batch []*account_manager.AddActivityRequest

			expire := time.After(maxTimeout)
			for {
				select {
				case req := <-ch:
					batch = append(batch, req)

					if len(batch) == maxItems {
						goto saveBatch
					}

				case <-expire:
					goto saveBatch
				}
			}

		saveBatch:
			if err := saveBatch(batch); err != nil {
				logger.Warnf("%v - NO ACTIVITY SAVED\n%+v", err, batch)
			}
		}
	}()

	go func() {
		// Send any pending activity...
		for {
			for {
				dir, _ := gocore.Config().Get("activity_batch_path", "./pending_activity")

				files, err := ioutil.ReadDir(dir)
				if err != nil {
					logger.Errorf("Could not read directory %s: %v - will retry activity processor", dir, err)
					goto sleep
				}

				if len(files) == 0 {
					goto sleep
				}

				for _, file := range files {
					filename := path.Join(dir, file.Name())

					if !strings.HasSuffix(filename, ".json") {
						continue
					}

					b, err := ioutil.ReadFile(filename)
					if err != nil {
						logger.Errorf("Could not read file %s: %v - will retry activity processor", filename, err)
						goto sleep
					}

					var batch []*account_manager.AddActivityRequest
					if err := json.Unmarshal(b, &batch); err != nil {
						logger.Errorf("Could not Unmarshal file %s: %v - will retry activity processor", filename, err)
						continue
					}

					if err := sendBatch(batch); err != nil {
						logger.Errorf("Could not sendBatch for activity for file %s: %v", filename, err)
						goto sleep
					}

					if err := os.Remove(filename); err != nil {
						logger.Errorf("Could not remove file %s: %v - STOPPING pending activity processor", filename, err)
						goto stop
					}
				}
			}
		sleep:
			time.Sleep(10 * time.Second) // TODO - setting
		}

	stop:
		// End goroutine
	}()
}

func sendBatch(batch []*account_manager.AddActivityRequest) error {
	ctx := context.Background()

	if len(batch) > 0 {
		con, err := getConn(ctx)
		if err == nil { // There was no error so we attempt to send the activity batch
			client := account_manager.NewAccountManagerClient(con)

			req := &account_manager.AddActivityBatchRequest{
				Requests: batch,
			}

			if _, err := client.AddActivityBatch(ctx, req); err != nil {
				return err
			}
		} else {
			logger.Errorf("SendBatch to Account Manager failed", err)
		}
	}

	return nil
}

func saveBatch(batch []*account_manager.AddActivityRequest) error {
	if len(batch) > 0 {
		b, err := json.MarshalIndent(&batch, "", "  ")
		if err != nil {
			return err
		}

		dir, _ := gocore.Config().Get("activity_batch_path", "./pending_activity")
		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}

		filename := fmt.Sprintf("activity_%s_%d.json", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), rand.Intn(99999))
		filename = path.Join(dir, filename)

		if err := ioutil.WriteFile(filename, b, 0666); err != nil {
			return err
		}
	}

	return nil
}

func Close() {
	if conn != nil {
		if err := conn.Close(); err != nil {
			logger.Warnf("failed to close account-manager connection: %v", err)
		}
	}
}
