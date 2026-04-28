package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/mitchellh/mapstructure"
	"github.com/teranode-group/common/bsdecoder"

	"github.com/ordishs/gocore"
)

var logger = gocore.Log("woc-api")

// OpReturn is a structure used for serializing/deserializing data in Elasticsearch.
type OpReturn struct {
	Tag     string    `json:"tag"`
	Subtag  string    `json:"subtag"`
	Fulltag string    `json:"fulltag"`
	Script  string    `json:"script"`
	TxID    string    `json:"txid"`
	Vout    int       `json:"vout"`
	Height  int       `json:"height"`
	Created time.Time `json:"created,omitempty"`
}

// Result comment
type Result struct {
	Term    string     `json:"term"`
	Count   int64      `json:"count"`
	From    int        `json:"from"`
	Size    int        `json:"size"`
	Results []OpReturn `json:"results"`
}

// TagCountResult comment
type TagCountResult struct {
	Count   int64      `json:"count"`
	Results []TagCount `json:"results"`
}

// TagCount comment
type TagCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

var (
	singleton            *elasticsearch.Client
	once                 sync.Once
	indexName            string
	blockSumaryIndexName string
)

const maxSize int = 100

// When aggregating terms in an elastic seatrch query we need to specify a max size
// as by default elastic search will only return the the top 10 terms
const MAX_TERMS_SIZE int = 1000

// Find method
func Find(q string, from int, size int) (result Result, err error) {
	var r map[string]interface{}
	if size > maxSize {
		size = maxSize
	}
	if len(q) < 3 {
		logger.Errorf("Query %s too short", q)
		return
	}

	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return
	}

	// Search with a term query
	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(`{
	  "query": {
		"bool": {
			"should": [
		        {
		          "term": {
		            "script": "%s"
		          }
		        },
		        {
		          "wildcard": {
		            "tag": "%s*"
		          }
		        }
			  ]
			}
			},
			"from": %d,
			"size": %d,
			"sort" : [
     			{"created" : {"order" : "desc"}}
   			]
		}`, q, q, from, size))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
		es.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		logger.Errorf("Find - Failed to run es query: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return
	}

	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		logger.Errorf("Error parsing the response body: %s", err)
		return
	}
	// Print the response status, number of results, and request duration.
	totalHits := int64(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64))

	stringToDateTimeHook := func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if t == reflect.TypeOf(time.Time{}) && f == reflect.TypeOf("") {
			return time.Parse("2006-01-02T15:04:05Z", data.(string))
		}

		return data, nil
	}
	var ttyp OpReturn
	config := mapstructure.DecoderConfig{
		DecodeHook: stringToDateTimeHook,
		Result:     &ttyp,
	}

	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return
	}

	// loop through hits
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {

		decoder.Decode(hit.(map[string]interface{})["_source"])
		result.Results = append(result.Results, ttyp)
	}

	if totalHits < 1 {
		return
	}
	result.Count = totalHits
	result.Term = q
	result.From = from
	result.Size = size

	return
}

// FindContentTypes method
func FindContentTypes(q []string, from int, size int) (result Result, err error) {
	if size > maxSize {
		size = maxSize
	}
	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return
	}

	qa := make([]string, 0)
	for _, pt := range q {
		qa = append(qa, fmt.Sprintf(`{"script":"%s"}`, pt))
	}

	qs := strings.Join(qa, ",")
	// Search with a term query
	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(`{
	  "query": {
		"bool": {
			"should": [
		       %s
			  ]
			}
			},
			"from": %d,
	  		"size": %d
		}`, qs, from, size))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
		es.Search.WithTrackTotalHits(true),
		es.Search.WithPretty(),
	)

	if err != nil {
		logger.Errorf("FindContentTypes - Failed to run es query: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return
	}

	var r map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		logger.Errorf("Error parsing the response body: %s", err)
		return
	}

	// Print the response status, number of results, and request duration.
	totalHits := int64(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64))

	result.Count = totalHits
	// 	// result.Term = q
	result.From = from
	result.Size = size

	// loop through hits
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		var ttyp OpReturn
		mapstructure.Decode(hit.(map[string]interface{})["_source"], &ttyp)
		result.Results = append(result.Results, ttyp)
	}

	if totalHits < 1 {
		return
	}

	return
}

// GetTagCount method
func GetTagCount(from string, to string) (result TagCountResult, err error) {
	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return
	}

	// Search with a term query
	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(`
			{
			  "size": 0,
			  "aggs": {
				"tags": {
				  "terms": {
					"field": "fulltag.keyword"
				  }
				}
			  },
			  "query": {
				"bool": {
				  "must": {
				   "range": {
						   "created": {
						  "format":"yyyy-MM-dd HH:mm:ss",
						  "from":"%s", 
						  "to":"%s"
						}
					  }
					}
				  }
			  }
			}`, from, to))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
		es.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		logger.Errorf("GetTagCount - Failed to run es query: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return
	}

	var r map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		logger.Errorf("Error parsing the response body: %s", err)
		return
	}

	totalHits := int64(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64))

	result.Count = totalHits

	// loop through hits
	buckets := r["aggregations"].(map[string]interface{})["tags"].(map[string]interface{})["buckets"].([]interface{})

	for _, b := range buckets {
		b2 := b.(map[string]interface{})
		result.Results = append(result.Results, TagCount{Name: b2["key"].(string), Count: int64(b2["doc_count"].(float64))})
	}

	return
}

func GetTagCountByBlockHeight(blockHeight string) (result TagCountResult, err error) {
	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return
	}

	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(`
			{
				"size":0,
				"aggs":{
				   "tags":{
					  "terms":{
						 "field":"fulltag.keyword",
						 "size": %d
					  }
				   }
				},
				"query":{
				   "bool":{
					  "should":[
						 {
							"term":{
							   "height":"%s"
							}
						 }
					  ]
				   }
				}
			 }`, MAX_TERMS_SIZE, blockHeight))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
		es.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		logger.Errorf("GetTagCountByBlockHeight - Failed to run es query: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return
	}

	var r map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		logger.Errorf("Error parsing the response body: %s", err)
		return
	}

	// loop through hits
	buckets := r["aggregations"].(map[string]interface{})["tags"].(map[string]interface{})["buckets"].([]interface{})

	totalHits := int64(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64))
	result.Count = totalHits

	for _, b := range buckets {
		b2 := b.(map[string]interface{})
		result.Results = append(result.Results, TagCount{Name: b2["key"].(string), Count: int64(b2["doc_count"].(float64))})
	}

	return
}

// GetTagsPerWeek method
func GetTagsPerWeek() (tagsPerWeek map[string]map[string]int, err error) {
	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return
	}

	// Search with a term query
	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(
				`{
				"aggs": {
				  "by_day": {
					"date_histogram": {
					  "field": "created",
					  "fixed_interval": "7d"
					},
					"aggs": {
					  "group_by_tags": {
						"terms": {
						  "field": "fulltag.keyword",
						  "size": %d
						}
					  }
					}
				  }
				},
				"query": {
				  "bool": {
					"filter": {
					  "range": {
						"created": {
						  "gte": "now-1223d/m",
						  "lte": "now/m"
						}
					  }
					}
				  }
				}
			  }`, MAX_TERMS_SIZE))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
	)

	if err != nil {
		logger.Errorf("GetTagsPerWeek - Failed to run es query: %s", err)
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return nil, err
	}

	var r map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		logger.Errorf("Error parsing the response body: %s", err)
		return nil, err
	}

	// loop through hits
	buckets := r["aggregations"].(map[string]interface{})["by_day"].(map[string]interface{})["buckets"].([]interface{})

	tagsPerWeek = make(map[string]map[string]int)

	for _, week := range buckets {
		weekMap := week.(map[string]interface{})

		tagCount := make(map[string]int)
		for _, v := range weekMap["group_by_tags"].(map[string]interface{})["buckets"].([]interface{}) {
			tagMap := v.(map[string]interface{})
			tagCount[tagMap["key"].(string)] = int(tagMap["doc_count"].(float64))
		}
		tagsPerWeek[weekMap["key_as_string"].(string)] = tagCount
	}

	return tagsPerWeek, nil
}

func SearchByFullTag(q string, from int, size int) (result Result, err error) {
	var r map[string]interface{}

	maxSize := 1000

	if size > maxSize {
		size = maxSize
	}

	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return
	}

	// Search with a term query
	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(`{
	  "_source": ["tag", "fulltag", "vout","created","txid","height"],
	  "query": {
	
		"bool": {
			"should": [
		        {
		          "term": {
		            "fulltag.keyword": "%s"
		          }
		        }
			  ]
			}
			},
			"from": %d,
			"size": %d,
			"sort" : [
     			{"created" : {"order" : "desc"}}
   			]
		}`, q, from, size))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
		es.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		logger.Errorf("Error: Find - Failed to run es query: %s", err)
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
			return
		}
	}

	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		logger.Errorf("Error parsing the response body: %s", err)
		return
	}
	// Print the response status, number of results, and request duration.
	totalHits := int64(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64))

	stringToDateTimeHook := func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if t == reflect.TypeOf(time.Time{}) && f == reflect.TypeOf("") {
			return time.Parse("2006-01-02T15:04:05Z", data.(string))
		}

		return data, nil
	}
	var ttyp OpReturn
	config := mapstructure.DecoderConfig{
		DecodeHook: stringToDateTimeHook,
		Result:     &ttyp,
	}

	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return
	}

	// loop through hits
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {

		decoder.Decode(hit.(map[string]interface{})["_source"])
		result.Results = append(result.Results, ttyp)
	}

	if totalHits < 1 {
		return
	}
	result.Count = totalHits
	result.Term = q
	result.From = from
	result.Size = size

	return
}

// GetTagsByRange method
func GetTagsByDays(days string) (result map[string]map[string]TagDetails, err error) {
	var dateRange = map[string]string{"1": "now-1d", "7": "now-7d", "30": "now-30d", "90": "now-90d"}

	var frequency = "day"
	if days == "1" {
		frequency = "1h" // Use hourly interval for one day
	} else {
		frequency = "1d" // Use daily interval for other durations
	}

	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return nil, err
	}

	// Search with a term query
	blockTagsRes, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(
				`{
					"size": 0,
					"query": {
						"bool": {
						"filter": {
							"range": {
							"created": {
								"gte": "%s",  
								"lte": "now"
							}
							}
						}
						}
					},
					"aggs": {
						"tags": {
						"terms": {
							"field": "fulltag.keyword",
							"size": %d
						},
						"aggs": {
							"timestamps": {
							"date_histogram": {
								"field": "created",
								"fixed_interval": "%s"
							},
							"aggs": {
								"total_txcount": {
								"sum": {
									"field": "txcount"
								}
								},
								"total_size": {
								"sum": {
									"field": "size"
								}
								},
								"total_output": {
								"sum": {
									"field": "total_output"
								}
								}
							}
							}
						}
						}
					}
				}
			`, dateRange[days], MAX_TERMS_SIZE, frequency))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(blockSumaryIndexName),
	)

	if err != nil {
		logger.Errorf("Error: GetTagsByDays from woc_block_summary_index - Failed to run es query: %s", err)
		return nil, err
	}

	defer func() {
		if blockTagsRes != nil {
			blockTagsRes.Body.Close()
		}
	}()

	if blockTagsRes.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(blockTagsRes.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				blockTagsRes.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return nil, err
	}

	var blockTagsData map[string]interface{}
	err = json.NewDecoder(blockTagsRes.Body).Decode(&blockTagsData)
	if err != nil {
		logger.Errorf("Error: GetTagsByDays from woc_block_summary index - Failed to run es query: %s", err)
		return nil, err
	}

	// Search with a term query
	tagsRes, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(
				`{
						"aggs": {
						  "by_day": {
							"date_histogram": {
							  "field": "created",
							  "fixed_interval": "%s"
							},
							"aggs": {
							  "group_by_tags": {
								"terms": {
								  "field": "fulltag.keyword",
								  "size": %d
								}
							  }
							}
						  }
						},
						"query": {
						  "bool": {
							"filter": {
							  "range": {
								"created": {
								  "gte": "%s",
								  "lte": "now"
								}
							  }
							}
						  }
						}
					  }`, frequency, MAX_TERMS_SIZE, dateRange[days]))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
	)

	if err != nil {
		logger.Errorf("Error: GetTagsByDays from prime index - Failed to run es query: %s", err)
		return nil, err
	}

	defer func() {
		if tagsRes != nil {
			tagsRes.Body.Close()
		}
	}()

	if tagsRes.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(tagsRes.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				tagsRes.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return
	}

	var tagsData map[string]interface{}
	err = json.NewDecoder(tagsRes.Body).Decode(&tagsData)
	if err != nil {
		logger.Errorf("Error: GetTagsByDays from woc_block_summary index - Failed to run es query: %s", err)
		return nil, err
	}

	// loop through hits
	buckets := tagsData["aggregations"].(map[string]interface{})["by_day"].(map[string]interface{})["buckets"].([]interface{})

	tagsByDays := make(map[string]map[string]int)

	for _, week := range buckets {
		daysMap := week.(map[string]interface{})

		tagCount := make(map[string]int)
		for _, v := range daysMap["group_by_tags"].(map[string]interface{})["buckets"].([]interface{}) {
			tagMap := v.(map[string]interface{})
			tagCount[tagMap["key"].(string)] = int(tagMap["doc_count"].(float64))
		}

		tagsByDays[daysMap["key_as_string"].(string)] = tagCount
	}

	// loop through hits
	buckets = blockTagsData["aggregations"].(map[string]interface{})["tags"].(map[string]interface{})["buckets"].([]interface{})

	for _, day := range buckets {
		dayMap := day.(map[string]interface{})

		tagKey := dayMap["key"].(string)

		for _, bucket := range dayMap["timestamps"].(map[string]interface{})["buckets"].([]interface{}) {

			bucketMap := bucket.(map[string]interface{})

			key := bucketMap["key_as_string"].(string)

			totalOutput := int64(bucketMap["total_output"].(map[string]interface{})["value"].(float64))
			totalSize := int64(bucketMap["total_size"].(map[string]interface{})["value"].(float64))
			totalTxcount := int64(bucketMap["total_txcount"].(map[string]interface{})["value"].(float64))

			if result == nil {
				result = make(map[string]map[string]TagDetails)
			}

			if _, ok := result[key]; !ok {
				result[key] = make(map[string]TagDetails)
			}

			if totalTxcount > 0 {

				if _, ok := result[key][tagKey]; !ok {
					result[key][tagKey] = TagDetails{}
				}

				outs := int64(tagsByDays[key][tagKey])

				//Temp patch until outs data is resolved as its incorrect on some blocks
				if totalTxcount > outs {
					outs = totalTxcount
				}

				result[key][tagKey] = TagDetails{
					TotalTxcount: totalTxcount,
					TotalOuts:    outs,
					TotalSize:    totalSize,
					TotalOutput:  totalOutput,
				}
			}
		}

	}

	return result, nil
}

type TagDetails struct {
	TotalOutput  int64 `json:"total_output"`
	TotalSize    int64 `json:"total_size"`
	TotalTxcount int64 `json:"total_txcount"`
	TotalOuts    int64 `json:"total_outs"`
}

func GetTagsSummaryByDays(days string) (tagDetailsList map[string]TagDetails, err error) {
	var dateRange = map[string]string{"1": "now-1d", "7": "now-7d", "30": "now-30d", "90": "now-90d"}

	es, err := getClient()
	if err != nil {
		logger.Errorf("error getting elastic client %+v", err)
		return nil, err
	}

	// Search with a term query
	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(
				`{
					"query": {
						"range": {
						"created": {
							"gte": "%s/d",
							"lte": "now/d"
						}
						}
					},
					"aggs": {
						"tags": {
						"terms": {
							"field": "fulltag.keyword",
							"size": %d
						},
						"aggs": {
							"total_txcount": {
							"sum": {
								"field": "txcount"
							}
							},
							"total_size": {
							"sum": {
								"field": "size"
							}
							},
							"total_output": {
							"sum": {
								"field": "total_output"
							}
							}
						}
						}
					},
					"size": 0
					}`, dateRange[days], MAX_TERMS_SIZE))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(blockSumaryIndexName),
	)

	if err != nil {
		logger.Errorf("Error: GetTagsSummaryByDays - Failed to run es query: %s", err)
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return nil, err
	}

	var r map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&r)

	if err != nil {
		logger.Errorf("Error: GetTagsSummaryByDays - Failed to run es query: %s", err)
		return nil, err
	}

	// loop through hits
	buckets := r["aggregations"].(map[string]interface{})["tags"].(map[string]interface{})["buckets"].([]interface{})

	tagDetailsList = make(map[string]TagDetails)

	for _, bucket := range buckets {
		bucketMap := bucket.(map[string]interface{})
		key := bucketMap["key"].(string)

		totalOutput := int64(bucketMap["total_output"].(map[string]interface{})["value"].(float64))
		totalSize := int64(bucketMap["total_size"].(map[string]interface{})["value"].(float64))
		totalTxcount := int64(bucketMap["total_txcount"].(map[string]interface{})["value"].(float64))

		tagDetailsList[key] = TagDetails{
			TotalOutput:  totalOutput,
			TotalSize:    totalSize,
			TotalTxcount: totalTxcount,
		}
	}
	return tagDetailsList, nil
}

func GetTagByTxIdVoutIndex(txId string, voutIndex int) (opTag *bsdecoder.OpReturn, err error) {
	opTag = &bsdecoder.OpReturn{}
	es, err := getClient()
	if err != nil {
		logger.Errorf("GetTagByTxidVoutIndex - Failed to connect es: %s", err)
		return nil, err
	}
	// Search with a term query
	res, err := es.Search(
		es.Search.WithBody(strings.NewReader(
			fmt.Sprintf(
				`
					{
						"size": 1,
						"aggs": {},
						"_source": false,
						"fields": [
						  "tag",
						  "subtag"
						],
						"query": {
						  "bool": {
							"must": [
							  {
								"match": {
								  "txid": "%s"
								}
							  },
							  {
								"term": {
								  "vout": %d
								}
							  }
							]
						  }
						}
					  }`, txId, voutIndex))),
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(indexName),
	)

	if err != nil {
		logger.Errorf("GetTagByTxidVoutIndex - Failed to run es query: %s", err)
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		err = json.NewDecoder(res.Body).Decode(&e)
		if err != nil {
			logger.Errorf("GetTagByTxidVoutIndex - Failed to parse the es error response body: %s", err)
		} else {
			// Print the response status and error information.
			logger.Errorf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
		return nil, err
	}

	var r map[string]interface{}
	err = json.NewDecoder(res.Body).Decode(&r)
	if err != nil {
		logger.Errorf("GetTagByTxidVoutIndex - Failed to parse the es response body: %s", err)
		return nil, err
	}

	// hits := r["hits"].(map[string]interface{})["hits"]

	hits := r["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(hits) > 0 {
		data := hits[0]
		if data != nil {
			tagMap := data.(map[string]interface{})
			opTag.Type = tagMap["fields"].(map[string]interface{})["tag"].([]interface{})[0].(string)
			opTag.Action = tagMap["fields"].(map[string]interface{})["subtag"].([]interface{})[0].(string)
		}
	}
	return opTag, nil
}

// getClient returns an elastic client
func getClient() (*elasticsearch.Client, error) {

	var err error
	once.Do(func() {
		ok := false
		indexName, ok = gocore.Config().Get("es_indexname")
		if !ok {
			logger.Fatal("Must have an elasticsearch indexname setting")
		}

		blockSumaryIndexName, ok = gocore.Config().Get("es_woc_block_summary_indexname")
		if !ok {
			logger.Fatal("Must have an elasticsearch indexname setting")
		}

		elasticsearchURL, _ := gocore.Config().Get("es_url")
		if elasticsearchURL == "" {
			logger.Fatal("Cannot start bitcoin scanner - No es_url")
		}

		elasticSearchURLs := strings.Split(elasticsearchURL, ",")

		httpClient := &http.Client{Timeout: 5 * time.Second}
		singleton, err = elasticsearch.NewClient(elasticsearch.Config{
			Addresses:    elasticSearchURLs,
			DisableRetry: true,
			Transport:    httpClient.Transport,
		})
		if err != nil {
			logger.Errorf("Error creating elastic search client: %v", err)
			return
		}
	})
	return singleton, err
}
