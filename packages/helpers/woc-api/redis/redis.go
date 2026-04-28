package redis

import (
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"time"

	// "github.com/teranode-group/woc-api/bitcoin"
	"github.com/gomodule/redigo/redis"
	"github.com/ordishs/gocore"
)

var logger = gocore.Log("woc-api")

const (
	defaultReadTimeout    = time.Second
	defaultWriteTimeout   = time.Second
	defaultConnectTimeout = time.Second
	defaultIdleTimeout    = 60 * time.Second
	defaultPoolSize       = 1500
)

type Conn = redis.Conn

type RedisConnPool interface {
	Get() redis.Conn
	Stats() redis.PoolStats
}

type RedisConfig struct {
	// Host is Redis server host.
	Host string
	// Port is Redis server port.
	Port int
	// ReadTimeout is a timeout on read operations.
	ReadTimeout time.Duration
	// WriteTimeout is a timeout on write operations.
	WriteTimeout time.Duration
	// ConnectTimeout is a timeout on connect operation.
	ConnectTimeout time.Duration
	// IdleTimeout is timeout after which idle connections to Redis will be closed.
	IdleTimeout time.Duration
	// DB is Redis database number. If not set then database 0 used.
	DB int
}

type Redis struct {
	ConnPool RedisConnPool
	Enabled  bool
}

var RedisClient Redis

// Lua script
var setScript = redis.NewScript(1, `redis.call("SET", KEYS[1], ARGV[1])`)
var setScriptWithExpSec = redis.NewScript(1, `redis.call("SET", KEYS[1], ARGV[1], "EX", ARGV[2])`)
var getScript = redis.NewScript(1, `return redis.call("GET", KEYS[1])`)
var decrementScript = redis.NewScript(1, `return redis.call('DECR', KEYS[1])`)
var decrementByScript = redis.NewScript(1, `return redis.call('DECRBY', KEYS[1], ARGV[1])`)
var deleteScript = redis.NewScript(1, `return redis.call('DEL', KEYS[1])`)
var ttlScript = redis.NewScript(1, `return redis.call("TTL", KEYS[1])`)

// https://pkg.go.dev/github.com/gomodule/redigo/redis#Pool
func makePoolFactory(conf RedisConfig) func(addr string, options ...redis.DialOption) (*redis.Pool, error) {

	poolSize := defaultPoolSize
	maxIdle := poolSize
	db := conf.DB
	return func(serverAddr string, dialOpts ...redis.DialOption) (*redis.Pool, error) {
		pool := &redis.Pool{
			MaxIdle:     maxIdle,
			MaxActive:   poolSize,
			Wait:        false,
			IdleTimeout: conf.IdleTimeout,
			Dial: func() (redis.Conn, error) {
				var c redis.Conn

				var err error
				c, err = redis.Dial("tcp", serverAddr, dialOpts...)
				if err != nil {
					logger.Errorf("error dialing to Redis: %+v", map[string]interface{}{"error": err.Error(), "addr": serverAddr})
					return nil, err
				}

				if db != 0 {
					if _, err := c.Do("SELECT", db); err != nil {
						_ = c.Close()
						logger.Errorf("error selecting Redis db: %+v", map[string]interface{}{"error": err.Error()})

						return nil, err
					}
				}

				return c, nil
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				_, err := c.Do("PING")
				return err
			},
		}
		return pool, nil
	}
}

func getDialOpts(conf RedisConfig) []redis.DialOption {
	var readTimeout = defaultReadTimeout
	if conf.ReadTimeout != 0 {
		readTimeout = conf.ReadTimeout
	}
	var writeTimeout = defaultWriteTimeout
	if conf.WriteTimeout != 0 {
		writeTimeout = conf.WriteTimeout
	}
	var connectTimeout = defaultConnectTimeout
	if conf.ConnectTimeout != 0 {
		connectTimeout = conf.ConnectTimeout
	}

	dialOpts := []redis.DialOption{
		redis.DialConnectTimeout(connectTimeout),
		redis.DialReadTimeout(readTimeout),
		redis.DialWriteTimeout(writeTimeout),
	}

	return dialOpts
}

func NewRedisCachePool(conf RedisConfig) (RedisConnPool, error) {
	host := conf.Host
	port := conf.Port

	poolFactory := makePoolFactory(conf)

	serverAddr := net.JoinHostPort(host, strconv.Itoa(port))
	pool, _ := poolFactory(serverAddr, getDialOpts(conf)...)
	return pool, nil
}

func Start() {

	cacheEnabled := gocore.Config().GetBool("redis_cache_enabled")
	RedisClient.Enabled = cacheEnabled

	if cacheEnabled {

		redisHost, ok := gocore.Config().Get("redis_host")
		if !ok {
			logger.Fatal("redis_host not found in settings")
		}
		redisPort, ok := gocore.Config().GetInt("redis_port")
		if !ok {
			logger.Fatal("redis_port not found in settings")
		}
		redisDB, ok := gocore.Config().GetInt("redis_db")
		if !ok {
			logger.Fatal("redis_port not found in settings")
		}

		logger.Infof("Starting Redis Cache %s:%v", redisHost, redisPort)

		var err error
		RedisClient.ConnPool, err = NewRedisCachePool(RedisConfig{
			Host:        redisHost,
			Port:        redisPort,
			DB:          redisDB,
			IdleTimeout: defaultIdleTimeout,
		})
		if err != nil {
			logger.Fatalf("Error: Unable to create resdis connection pool: %+v", err)
		}
	} else {
		logger.Info("Starting with Redis Cache Disabled")
	}
}

func GetCachedValue(key string, value interface{}, conn redis.Conn) error {

	if !RedisClient.Enabled {
		return errors.New("error: Redis cache not enabled")
	}

	//if Connection is not provided get one from the pool
	if conn == nil {
		conn = RedisClient.ConnPool.Get()
		defer conn.Close()
	}
	reply, err := getScript.Do(conn, key)
	if err != nil {
		logger.Errorf("GetCachedValue Err: %+v, Key: %+s, Reply: %+s", err, key, reply)
		return err
	}

	if reply == nil {
		return errors.New("not found")
	}

	bytes, err := redis.Bytes(reply, err)
	if err != nil {
		logger.Errorf("GetCachedValue bytes conversion Err: %+v, Key: %+s, Reply: %+s", err, key, reply)
		return err
	}

	return json.Unmarshal(bytes, value)
}

func SetCacheValue(key string, value interface{}, conn redis.Conn) error {

	if !RedisClient.Enabled {
		return errors.New("error: Redis cache not enabled")
	}

	v, err := json.Marshal(value)
	if err != nil {
		return err
	}

	//if Connection is not provided get one from the pool
	if conn == nil {
		conn = RedisClient.ConnPool.Get()
		_, err = setScript.Do(conn, key, v)
		conn.Flush()
		conn.Close()
	} else {
		_, err = setScript.Do(conn, key, v)
	}

	if err != nil {
		logger.Errorf("SetCacheValue %+v", err)
		return err
	}

	return nil

}

func SetCacheValueWithExpire(key string, value interface{}, seconds int64, conn redis.Conn) error {

	if !RedisClient.Enabled {
		return errors.New("error: Redis cache not enabled")
	}

	v, err := json.Marshal(value)
	if err != nil {
		return err
	}

	//if Connection is not provided get one from the pool
	if conn == nil {
		conn = RedisClient.ConnPool.Get()
		_, err = setScriptWithExpSec.Do(conn, key, v, seconds)
		conn.Flush()
		conn.Close()
	} else {
		_, err = setScriptWithExpSec.Do(conn, key, v, seconds)
	}

	if err != nil {
		logger.Errorf("SetCacheValueWithExpire %+v", err)
		return err
	}

	return nil
}

// Decrements the number stored at key by one. If the key does not exist, it is set to 0 before performing the operation.
func DecrementCachedValue(key string, conn redis.Conn) (value int64, err error) {

	if !RedisClient.Enabled {
		return -1, errors.New("error: Redis cache not enabled")
	}

	//if Connection is not provided get one from the pool
	if conn == nil {
		conn = RedisClient.ConnPool.Get()
		defer conn.Close()
	}
	reply, err := decrementScript.Do(conn, key)
	if err != nil {
		logger.Errorf("DecrementCachedValue Err: %+v, Key: %+s, Reply: %+s", err, key, reply)
		return -1, err
	}

	return reply.(int64), nil
}

// Decrements the number stored at key by one. If the key does not exist, it is set to 0 before performing the operation.
func DecrementByCachedValue(key string, count int, conn redis.Conn) (value int64, err error) {

	if !RedisClient.Enabled {
		return -1, errors.New("error: Redis cache not enabled")
	}

	//if Connection is not provided get one from the pool
	if conn == nil {
		conn = RedisClient.ConnPool.Get()
		defer conn.Close()
	}
	reply, err := decrementByScript.Do(conn, key, count)
	if err != nil {
		logger.Errorf("DecrementCachedValue Err: %+v, Key: %+s, Reply: %+s", err, key, reply)
		return -1, err
	}

	return reply.(int64), nil
}

func GetKeyTTLValue(key string, conn redis.Conn) (value int64, err error) {

	if !RedisClient.Enabled {
		return -1, errors.New("error: Redis cache not enabled")
	}

	//if Connection is not provided get one from the pool
	if conn == nil {
		conn = RedisClient.ConnPool.Get()
		defer conn.Close()
	}
	reply, err := ttlScript.Do(conn, key)
	if err != nil {
		logger.Errorf("GetTTLValue Err: %+v, Key: %+s, Reply: %+s", err, key, reply)
		return -1, err
	}

	return reply.(int64), nil
}

func DeleteKey(key string, conn redis.Conn) (value int64, err error) {

	if !RedisClient.Enabled {
		return -1, errors.New("error: Redis cache not enabled")
	}

	//if Connection is not provided get one from the pool
	if conn == nil {
		conn = RedisClient.ConnPool.Get()
		defer conn.Close()
	}
	reply, err := deleteScript.Do(conn, key)
	if err != nil {
		logger.Errorf("DeleteKey Err: %+v, Key: %+s, Reply: %+s", err, key, reply)
		return -1, err
	}

	return reply.(int64), nil
}
