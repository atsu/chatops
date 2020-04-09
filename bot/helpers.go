/*
The contents of this file should be all thread safe Global functions for use with templates
*/
package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/olivere/elastic"
	"github.com/zserge/metric"
)

// HelperMetrics is an object to make instrumenting what happens in the helper functions easier
var HelperMetrics = &helperInstrument{
	slowRequestThresh:             time.Second * 3,
	elasticSearchSlowRequests:     make([]slowRequest, 0, 20),
	elasticSearchResponseTimeSecs: metric.NewHistogram("1h1h"), // 1 hour history, 1 hour precision
	elasticSearchRequestCount:     metric.NewCounter("1h1h"),   // 1 hour history, 1 hour precision
}

type slowRequest struct {
	Timestamp    int64
	DurationSecs float64
	Url          string
	Body         string
}
type helperInstrument struct {
	slowRequestThresh time.Duration

	elasticSearchSlowRequests     []slowRequest
	elasticSearchResponseTimeSecs metric.Metric
	elasticSearchRequestCount     metric.Metric

	esRoundTripper http.RoundTripper
	lock           sync.RWMutex
}

func (h *helperInstrument) recordSlowRequest(duration time.Duration, url, body string) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if len(h.elasticSearchSlowRequests) >= 20 {
		h.elasticSearchSlowRequests = h.elasticSearchSlowRequests[:19]
	}
	h.elasticSearchSlowRequests = append([]slowRequest{{
		DurationSecs: duration.Seconds(),
		Url:          url,
		Body:         body,
		Timestamp:    time.Now().Unix(),
	}}, h.elasticSearchSlowRequests...)
}

// RoundTrip used to instrument elastic search requests https://github.com/olivere/elastic/wiki/HttpTransport
func (h *helperInstrument) RoundTrip(r *http.Request) (*http.Response, error) {
	h.elasticSearchRequestCount.Add(1)
	start := time.Now()
	var body []byte
	var err error
	if r.Body != nil {
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println("error reading request body:", err)
		} else {
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
		}
		if err := r.Body.Close(); err != nil {
			log.Println("error closing body:", err)
		}
	}
	defer func() {
		since := time.Since(start)
		if since > h.slowRequestThresh {
			h.recordSlowRequest(since, r.URL.String(), string(body))
		}
		h.elasticSearchResponseTimeSecs.Add(since.Seconds())
	}()
	return http.DefaultTransport.RoundTrip(r)
}

type HelperStatus struct {
	ElasticSearchResponseTimeSecs interface{}
	ElasticSearchRequestCount     interface{}
	ElasticSearchSlowRequests     interface{}
}

// Status returns a HelperStatus object to encapsulate the current status of the helpers
func (h *helperInstrument) Status() HelperStatus {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return HelperStatus{
		ElasticSearchRequestCount:     h.elasticSearchRequestCount,
		ElasticSearchResponseTimeSecs: h.elasticSearchResponseTimeSecs,
		ElasticSearchSlowRequests:     h.elasticSearchSlowRequests,
	}
}

/* ===== Helpers Below =====*/

func CachedByKey(k string) string {
	if val, ok := GlobalCache.GetByKey(k); ok {
		return val.(string)
	}
	return ""
}

func CachedByVal(v string) string {
	if key, ok := GlobalCache.GetByValue(v); ok {
		return key.(string)
	}
	return ""
}

func GetMounts(host, index string) ([]string, error) {
	es, err := elastic.NewSimpleClient(
		elastic.SetURL(host),
		elastic.SetHttpClient(&http.Client{Transport: HelperMetrics}),
	)
	if err != nil {
		return nil, err
	}
	res, err := es.Search(index).Aggregation("mounts",
		elastic.NewTermsAggregation().Field("mnt.keyword").Size(10000000)).
		Do(context.Background())
	if err != nil {
		return nil, err
	}
	b := res.Aggregations["mounts"]
	var ki elastic.AggregationBucketKeyItems
	if err := json.Unmarshal(*b, &ki); err != nil {
		return nil, err
	}
	mounts := make([]string, 0)
	for _, bucket := range ki.Buckets {
		if str, ok := bucket.Key.(string); ok {
			key := uuid.New().String()
			GlobalCache.Add(key, str)
			mounts = append(mounts, str)
		}
	}
	return mounts, nil
}

func GetAnomalies() string {
	if rand.Int()%2 == 0 {
		return "Jid:132 - anomaly score: .05"
	}
	return ""
}

// TruncatePath is a helper for trimming paths to a specific size limit
// we try to keep it pretty by chopping off inner path segments
// eg '/one/two/three/four/five' becomes '/one/*/five' for 11 char limit
//
// if the last segment is > char limit, we simply truncate the name
// eg '/one/two/somelongfilename' becomes '*name' for 5 char limit
func TruncatePath(p string, max int) string {
	if max < 1 {
		return ""
	}
	if len(p) > max {
		base, mnt := path.Split(p)
		if len(mnt) > max {
			start := len(mnt) - (max - 1)
			if start > 0 {
				return "*" + mnt[start:]
			}
		}

		out := ""
		for {
			base = path.Dir(base)
			out = path.Join(base, "*", mnt)
			if len(out) <= max {
				return out
			}
		}
	}
	return p
}

// Error is a simple function to allow throwing errors from within templates
func Error(msg string) (bool, error) {
	return false, errors.New(msg)
}
