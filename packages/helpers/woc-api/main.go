package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/teranode-group/woc-api/activitystore"
	"github.com/teranode-group/woc-api/apikeys"
	"github.com/teranode-group/woc-api/bstore"
	"github.com/teranode-group/woc-api/utxosmempool"
	"github.com/teranode-group/woc-api/utxostore"

	commongrpc "github.com/teranode-group/common/grpc"
	"github.com/teranode-group/common/logger"
	"github.com/teranode-group/common/profiler"
	p2p_service "github.com/teranode-group/proto/p2p-service"
	woc_stats "github.com/teranode-group/proto/woc-stats"

	woc_exchange_rate "github.com/teranode-group/proto/woc-exchange-rate"
	"github.com/teranode-group/woc-api/configs"
	"github.com/teranode-group/woc-api/redis"
	"github.com/teranode-group/woc-api/serverfiber"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	_ "github.com/lib/pq"

	"github.com/ordishs/gocore"
	"github.com/teranode-group/woc-api/server"
)

// Name used by build script for the binaries. (Please keep on single line)
const progname = "woc-api"

// Version & commit strings injected at build with -ldflags -X...
var version string
var commit string

var (
	wocStatsConnection        *grpc.ClientConn
	wocExchangeRateConnection *grpc.ClientConn
	p2pServiceConnection      *grpc.ClientConn
	serverFiber               *serverfiber.Server
)

func main() {
	gocore.SetInfo(progname, version, commit)

	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	if err := configs.Load(); err != nil {
		log.Fatalf("failed to get config: %s", err.Error())
	}

	err := logger.NewZapLogger(
		version,
		commit,
		progname,
		configs.Settings.LogLevel,
		configs.Settings.PrettifyLog,
	)
	if err != nil {
		log.Fatalf("failed to start logger: %s", err.Error())
	}
	defer logger.Log.Sync() //nolint: errcheck

	go profiler.Start(ctx, logger.Log, configs.Settings.ProfilerAddress)

	go func() {
		stop := <-stop
		logger.Log.Info("signal to stop service detected", zap.String("signal", stop.String()))
		cancel()
	}()

	stats := gocore.Config().Stats()
	log.Printf("STATS\n%s\nVERSION\n-------\n%s (%s)\n\n", stats, version, commit)

	if err = bootstrap(); err != nil {
		logger.Log.Error("bootstrap failure", zap.Error(err))
		cancel()
		return
	}
	defer shutdown()

	startServer(cancel)
	<-ctx.Done()
}

func startServer(cancel context.CancelFunc) {

	//Initialize redis cache
	redis.Start()

	go server.Start()

	// POC for new endpoints using fiber
	// we are not calling it directly, but using proxy on server/server.go
	go func() {
		if err := serverFiber.Start(); err != nil {
			logger.Log.Error("fiber failure", zap.Error(err))
			cancel()
		}
	}()
}

func bootstrap() error {
	var err error
	wocStatsConnection, err = commongrpc.NewClientConnection(configs.Settings.WocStatsAddress, logger.Log, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to bstore %w", err)
	}

	wocExchangeRateConnection, err = commongrpc.NewClientConnection(configs.Settings.WocExchangeRateAddress, logger.Log, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to woc-exchnage-rate %w", err)
	}

	p2pServiceConnection, err = commongrpc.NewClientConnection(configs.Settings.P2pServiceAddress, logger.Log, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to p2p-service %w", err)
	}

	serverFiber = serverfiber.New(woc_stats.NewWocStatsClient(wocStatsConnection), woc_exchange_rate.NewWocExchangeRateClient(wocExchangeRateConnection), p2p_service.NewP2PServiceClient(p2pServiceConnection))

	return nil
}

func shutdown() {
	logger.Log.Info("shutting down grpc connections")

	if wocStatsConnection != nil {
		if err := wocStatsConnection.Close(); err != nil {
			logger.Log.Warn("failed to close woc-stats connection", zap.Error(err))
		}
	}

	if wocExchangeRateConnection != nil {
		if err := wocExchangeRateConnection.Close(); err != nil {
			logger.Log.Warn("failed to close woc-exchange-rate connection", zap.Error(err))
		}
	}

	if p2pServiceConnection != nil {
		if err := p2pServiceConnection.Close(); err != nil {
			logger.Log.Warn("failed to close p2p-service connection", zap.Error(err))
		}
	}

	bstore.Close()
	utxosmempool.Close()
	utxostore.Close()
	apikeys.Close()
	activitystore.Close()
}
