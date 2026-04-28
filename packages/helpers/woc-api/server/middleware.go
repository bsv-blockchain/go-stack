package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	account_manager "github.com/teranode-group/proto/account-manager"
	"github.com/teranode-group/woc-api/activitystore"
	"github.com/teranode-group/woc-api/apikeys"
	"github.com/teranode-group/woc-api/redis"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type keyLimits struct {
	allowedPerDay   bool
	remainingPerDay int64
	allowedPerSec   bool
	remainingPerSec int64
	restTimePerSec  int64
}

type ContextKey int

const (
	ContextDetails ContextKey = iota
	ContextStart
	ContextSizings
	ContextRPCMethod
)

const (
	// HeaderRateLimitLimit, HeaderRateLimitRemaining, and HeaderRateLimitReset
	// are the recommended return header values from IETF on rate limiting. Reset
	// is in UTC time.
	HeaderRateLimitLimit          = "X-RateLimit-Limit"
	HeaderRateLimitRemaining      = "X-RateLimit-Remaining"
	HeaderDailyRateLimit          = "X-RateLimit-Daily-Limit"
	HeaderDailyRateLimitRemaining = "X-RateLimit-Daily-Remaining"

	// HeaderRetryAfter is the header used to indicate when a client should retry
	// requests (when the rate limit expires), in UTC time.
	HeaderRetryAfter            = "Retry-After"
	RateLimitKeyExpiryInSeconds = 120
)

func TaalAPIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !apiKeyCheckEnabled {
			next.ServeHTTP(w, req)
		} else {
			apiKey := req.Header.Get("authorization")

			if len(apiKey) != 0 {
				apiKey = strings.Replace(apiKey, "Bearer ", "", 1)
			}

			if apiKey != "" {

				var details *account_manager.APIKey
				if offlineMode {
					details = apikeys.GetFromCache(apiKey)
				} else {
					details = apikeys.Get(apiKey)
				}
				if details == nil {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode("Invalid APIKey")
					return
				}

				if !details.IsAccountActive {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode("Account not active")
					return
				}

				if details.Revoked != nil {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode("APIKey not active (revoked)")
					return
				}

				if details.AccountId == 1 {
					// This is the _PUBLIC_ apikey. Reject...
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode("Please register at https://console.taal.com for a free API key.")
					return
				}

				timeNow := time.Now()

				if details.Expiry != nil && details.Expiry.AsTime().Before(timeNow) {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode("Account not active (expired)")
					return
				}

				// using other network key for mainnet
				if details.Network != "mainnet" && isMainnet {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode("Incorrect network key")
					return
				}

				// using mainnet key for other networks
				if details.Network == "mainnet" && !isMainnet {
					w.WriteHeader(http.StatusUnauthorized)
					json.NewEncoder(w).Encode("Incorrect network key")
					return
				}

				ctx := req.Context()
				ctx = context.WithValue(ctx, ContextStart, timeNow)
				ctx = context.WithValue(ctx, ContextDetails, details)

				var responseCode = "429"

				if apiKeyRateLimitEnabled {
					maximumRequestsPerSec := details.WocRateLimit
					maximumRequestsPerDay := details.WocDailyRateLimit

					//TODO: review this business rule
					if !details.WocAccess || maximumRequestsPerSec <= 0 {
						maximumRequestsPerSec = int64(apiKeyDefaultRateLimitPerSec)
					}

					if !details.WocAccess || maximumRequestsPerDay <= 0 {
						maximumRequestsPerDay = int64(apiKeyDefaultRateLimitPerDay)
					}

					var intervalInSeconds int64 = 1
					key := apiKey
					weight := 1

					if endpointWeightEnabled {
						endpointPath := strings.ToLower(req.RequestURI)

						for key, value := range endpointWeightMap {
							if strings.Contains(endpointPath, key) {
								weight = value
								break
							}
						}
					}

					limits := rateLimitUsingTokenBucket(key, intervalInSeconds, maximumRequestsPerSec, maximumRequestsPerDay, weight, timeNow)

					if limits.allowedPerDay {

						if limits.allowedPerSec {
							// set headers
							w.Header().Set(HeaderDailyRateLimit, strconv.FormatInt(maximumRequestsPerDay, 10))
							w.Header().Set(HeaderDailyRateLimitRemaining, strconv.FormatInt(limits.remainingPerDay, 10))
							w.Header().Set(HeaderRateLimitLimit, strconv.FormatInt(maximumRequestsPerSec, 10))
							w.Header().Set(HeaderRateLimitRemaining, strconv.FormatInt(limits.remainingPerSec, 10))

							//for now consider anything served as 200
							responseCode = "200"
							next.ServeHTTP(w, req.WithContext(ctx))
						} else {
							// set header
							w.Header().Set(HeaderRetryAfter, time.Unix(0, int64(limits.restTimePerSec)).UTC().Format(time.RFC1123))

							w.WriteHeader(http.StatusTooManyRequests)
							json.NewEncoder(w).Encode(http.StatusText(http.StatusTooManyRequests))
						}

					} else {
						// set header
						midnight := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 00, 00, 00, 0, timeNow.Location())
						timeRetry := midnight.AddDate(0, 0, 1)
						w.Header().Set(HeaderRetryAfter, timeRetry.UTC().Format(time.RFC1123))

						key := key[0:11] + "..." + key[len(key)-3:]
						logger.Warnf("TaalAPIKeyMiddleware: Daily limit hit for APIKey %s", key)

						w.WriteHeader(http.StatusTooManyRequests)
						json.NewEncoder(w).Encode(http.StatusText(http.StatusTooManyRequests))
					}

				} else {
					responseCode = "200"
					next.ServeHTTP(w, req.WithContext(ctx))
				}

				if apiKeySaveActivity {

					a := &account_manager.AddActivityRequest{
						Timestamp:    timestamppb.New(timeNow),
						ApiKeyId:     details.ApikeyId,
						Network:      details.Network,
						ActivityType: "woc",
						SourceIp:     GetSourceIP(req),
						ResponseCode: responseCode,
					}

					activitystore.AddActivity(a)
				}

			} else {
				// no key requests or internal request
				// assuming nginx is doing rate limiting (old keys) for external no key requests.
				next.ServeHTTP(w, req)
			}
		}
	})
}

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// default type, overriden in the handlers if required
		w.Header().Add("Content-Type", "application/json; charset=UTF-8")

		// normally for websites but some clients calling directly from the client side - just in case
		//https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Content-Type-Options
		w.Header().Add("X-Content-Type-Options", "nosniff")

		//https: //developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Referrer-Policy
		w.Header().Add("Referrer-Policy", "origin")

		next.ServeHTTP(w, r)
	})
}

// TODO: param type
// RateLimitUsingTokenBucket - using redis
func rateLimitUsingTokenBucket(uniqueKey string, intervalInSeconds int64, maximumRequestsperSec int64, maximumRequestsPerDay int64, weight int, timeNow time.Time) (limits keyLimits) {
	if !redis.RedisClient.Enabled {
		logger.Fatal("Error: apiKeyRateLimitEnabled is set to true in settings which depends on redis. Redis is Disable!")
	}
	result := keyLimits{}

	//per sec
	counterKey := "taalkey_" + uniqueKey + "_counter"
	lastResetTimeKey := "taalkey_" + uniqueKey + "_last_reset_time"
	//per day
	dailyCounterKey := "taalkey_" + uniqueKey + "_daily_counter"

	result.allowedPerDay = false
	result.allowedPerSec = false
	result.remainingPerDay = 0
	result.remainingPerSec = 0
	result.restTimePerSec = 0

	redisConn := redis.RedisClient.ConnPool.Get()
	defer redisConn.Close()

	//per day limit check
	var dailyCounter int64
	err := redis.GetCachedValue(dailyCounterKey, &dailyCounter, redisConn)

	if err != nil {

		if strings.Contains(err.Error(), "not found") {

			timeExpiry := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 23, 59, 59, 0, timeNow.Location())
			difference := timeExpiry.Sub(timeNow)

			dailyCounter = maximumRequestsPerDay

			// if end of the day just renew for whole day
			ttlSeconds := int64(difference.Seconds())
			if ttlSeconds <= 60 {
				ttlSeconds = 86400
			}

			redis.SetCacheValueWithExpire(dailyCounterKey, dailyCounter, ttlSeconds, redisConn)
		} else {
			logger.Errorf("RateLimitUsingTokenBucket: getting dailyCounterKey - %v\n", err)
			return result
		}
	}

	if dailyCounter <= 0 {
		var dailyCounterTTL int64

		//check ttl is valid
		dailyCounterTTL, err = redis.GetKeyTTLValue(dailyCounterKey, redisConn)
		if err != nil {
			logger.Errorf("GetKeyTTLValue: getting dailyCounterKey - %v\n", err)
		}

		key := uniqueKey[0:11] + "..." + uniqueKey[len(uniqueKey)-3:]
		logger.Warnf("RateLimitUsingTokenBucket: daily_counter 0 for APIKey, %s, %d", key, dailyCounterTTL)

		//self healing - something went wrong if TTL is in negative
		//TODO:Mo: testing a key with issue. to be removed
		if dailyCounterTTL < 0 || key == "mainnet_f6b...a7b" {
			logger.Warnf("RateLimitUsingTokenBucket: daily_counter TTL for APIKey %s is below 0", key)

			redis.DeleteKey(dailyCounterKey, redisConn)
			redis.DeleteKey(lastResetTimeKey, redisConn)
			redis.DeleteKey(counterKey, redisConn)
		}

		return result
	} else if dailyCounter > maximumRequestsPerDay {

		// assuming maximumRequestsPerDay is readjusted (reduced)
		key := uniqueKey[0:11] + "..." + uniqueKey[len(uniqueKey)-3:]
		logger.Warnf("RateLimitUsingTokenBucket: readjusted maximumRequestsPerDay for key %s to %v\n", key, maximumRequestsPerDay)

		timeExpiry := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 23, 59, 59, 0, timeNow.Location())
		difference := timeExpiry.Sub(timeNow)

		dailyCounter = maximumRequestsPerDay

		ttlSeconds := int64(difference.Seconds())
		if ttlSeconds <= 30 {
			ttlSeconds = 86400
		}

		redis.SetCacheValueWithExpire(dailyCounterKey, dailyCounter, ttlSeconds, redisConn)
	}

	result.allowedPerDay = true

	//per sec limit check
	var lastResetTime int64

	err = redis.GetCachedValue(lastResetTimeKey, &lastResetTime, redisConn)

	if err != nil && !strings.Contains(err.Error(), "not found") {
		logger.Errorf("RateLimitUsingTokenBucket: getting lastResetTimeKey - %v\n", err)
		return result
	}

	// first request in window, lastResetTime will be set to current time and counter be set to max requests allowed
	// check if time window since last counter reset has elapsed
	if time.Now().Unix()-lastResetTime >= intervalInSeconds {
		// if elapsed, reset the counter & lastResetTime
		lastResetTime = time.Now().Unix()
		err := redis.SetCacheValueWithExpire(lastResetTimeKey, lastResetTime, RateLimitKeyExpiryInSeconds, redisConn)

		if err != nil {
			logger.Errorf("RateLimitUsingTokenBucket: setting lastResetTimeKey - %v\n", err)
			return result
		}
		redis.SetCacheValueWithExpire(counterKey, maximumRequestsperSec, RateLimitKeyExpiryInSeconds, redisConn)

		if err != nil {
			logger.Errorf("RateLimitUsingTokenBucket: setting maximumRequests - %v\n", err)
			return result
		}

	} else {
		var requestLeft int64

		err := redis.GetCachedValue(counterKey, &requestLeft, redisConn)
		if err != nil {
			logger.Errorf("RateLimitUsingTokenBucket: getting requestLeft - %v\n", err)
		}
		if requestLeft <= 0 { // request left is 0 or < 0
			result.restTimePerSec = lastResetTime + intervalInSeconds
			return result
		}
	}

	result.allowedPerSec = true

	if weight == 1 {
		// decrement daily request count by 1
		result.remainingPerDay, err = redis.DecrementCachedValue(dailyCounterKey, redisConn)
		if err != nil {
			logger.Errorf("RateLimitUsingTokenBucket: DecrementCachedValue - %v\n", err)
		}
	} else {
		result.remainingPerDay, err = redis.DecrementByCachedValue(dailyCounterKey, weight, redisConn)
		if err != nil {
			logger.Errorf("RateLimitUsingTokenBucket: DecrementCachedValue - %v\n", err)
		}
	}

	// decrement per sec request count by 1
	result.remainingPerSec, err = redis.DecrementCachedValue(counterKey, redisConn)
	if err != nil {
		logger.Errorf("RateLimitUsingTokenBucket: DecrementCachedValue - %v\n", err)
	}

	redisConn.Flush()

	return result
}

func GetSourceIP(req *http.Request) string {
	ipAddress := req.RemoteAddr

	fwdAddress := req.Header.Get("X-Forwarded-For") // capitalisation doesn't matter
	if fwdAddress != "" {
		// Got X-Forwarded-For

		// If we got an array... grab the first IP
		ips := strings.Split(fwdAddress, ", ")
		if len(ips) > 1 {
			return ips[0]
		}

		return fwdAddress
	}

	return ipAddress
}
