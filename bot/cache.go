package bot

import (
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultExpiryCapacity = 500
)

// TwoWayCache is a cache where you can retrieve values by key as well as key by value
// this implementation is fully thread safe, for use in asynchronous template execution
type TwoWayCache struct {
	m1       sync.Map
	m2       sync.Map
	seen     sync.Map
	expiry   []entry
	explock  sync.Mutex
	capacity int

	ttl time.Duration

	run    atomic.Value
	lastex time.Time
}

// GlobalCache always gets initialized and can be used simply with no other initialization
var GlobalCache = NewCache(time.Minute, defaultExpiryCapacity)

type entry struct {
	born  time.Time
	seen  int
	value interface{}
}

// NewCache create a new instance, takes in the entry cache lifetime, and the default capacity.
// the cache will begin to expire items once the capacity is reached.
func NewCache(duration time.Duration, capacity int) TwoWayCache {
	twc := TwoWayCache{
		ttl:      duration,
		capacity: capacity,
		expiry:   make([]entry, 0, capacity),
	}
	twc.run.Store(false)
	return twc
}

func (t *TwoWayCache) expire() {
	if t.run.Load().(bool) {
		return
	}
	go func() {
		t.run.Store(true)
		t.explock.Lock()
		defer func() {
			t.explock.Unlock()
			t.run.Store(false)
		}()
		if t.timeToExpire() {
			t.expireOldEntries()
		}
	}()
}

func (t *TwoWayCache) timeToExpire() bool {
	// over capacity and we haven't tried to expire since last time
	return len(t.expiry) > t.capacity && time.Since(t.lastex) > t.ttl
}

func (t *TwoWayCache) expireOldEntries() {
	expired := -1 // index of last expired
	for i := 0; i < len(t.expiry); i++ {
		exp := t.expiry[i]
		// only expire entries that have been around more than ttl
		if time.Since(exp.born) < t.ttl {
			break
		}
		key := exp.value
		last, ok := t.seen.Load(key)
		if !ok || exp.born.Unix() == last.(int64) {
			if val, ok := t.m1.Load(key); ok {
				t.m1.Delete(key)
				t.m2.Delete(val)
				t.seen.Delete(key)
				expired = i
			}
		}
	}
	if expired > -1 {
		// since t.expiry is a queue ordered by date, we can drop everything before the last expired index
		t.expiry = t.expiry[expired+1:]
		t.lastex = time.Now()
	}
}

// Add a new key value pair to the cache
func (t *TwoWayCache) Add(key, val interface{}) {
	defer t.expire()

	now := time.Now()
	if oldval, ok := t.m1.Load(key); ok {
		if reflect.DeepEqual(val, oldval) {
			// key pair already exists, update the seen map
			t.seen.Store(key, now.Unix())

			return
		} else {
			// clean up old value when an existing key is set
			// this will leave some dangling expiry entries, but they should just fall off after they expire.
			t.m2.Delete(oldval)
		}
	} else {
		t.m1.Store(key, val)
		t.m2.Store(val, key)
		t.seen.Store(key, now.Unix())
	}
	t.addExpiry(now, key)
}

func (t *TwoWayCache) addExpiry(tm time.Time, val interface{}) {
	t.explock.Lock()
	t.expiry = append(t.expiry, entry{born: tm, value: val})
	t.explock.Unlock()
}

// GetByKey retrieve the value by the supplied key
func (t *TwoWayCache) GetByKey(key interface{}) (interface{}, bool) {
	defer t.expire()

	now := time.Now()
	if v, ok := t.m1.Load(key); ok {
		t.seen.Store(key, now.Unix())
		t.addExpiry(now, key)
		return v, true
	}
	return nil, false
}

// GetByValue retrieve the key by the supplied value
func (t *TwoWayCache) GetByValue(val interface{}) (interface{}, bool) {
	defer t.expire()

	now := time.Now()
	if k, ok := t.m2.Load(val); ok {
		t.seen.Store(k, time.Now().Unix())
		t.addExpiry(now, k)
		return k, true
	}
	return nil, false
}

// Clear completely resets the cache
func (t *TwoWayCache) Clear() {
	t.m1 = sync.Map{}
	t.m2 = sync.Map{}
	t.seen = sync.Map{}

	t.explock.Lock()
	t.expiry = make([]entry, 0, defaultExpiryCapacity)
	t.explock.Unlock()
}
