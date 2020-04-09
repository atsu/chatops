package bot

import (
	"math/rand"
	"testing"
	"time"

	"github.com/atsu/goat/util"
	"github.com/stretchr/testify/assert"
)

func TestGlobalCacheTest(t *testing.T) {
	key := util.RandomString(10)
	val := util.RandomString(10)

	GlobalCache.Add(key, val)

	got, _ := GlobalCache.GetByKey(key)
	assert.Equal(t, val, got)

	got, _ = GlobalCache.GetByValue(val)
	assert.Equal(t, key, got)

	// No expiration testing, because the default timeout if 5 min :D

	GlobalCache.Add("one", "one")
	GlobalCache.Add("two", "two")
	GlobalCache.Add("three", "three")

	GlobalCache.Clear()
	one, _ := GlobalCache.GetByKey("one")
	assert.Empty(t, one)
	two, _ := GlobalCache.GetByKey("two")
	assert.Empty(t, two)
	three, _ := GlobalCache.GetByKey("three")
	assert.Empty(t, three)
	one, _ = GlobalCache.GetByValue("one")
	assert.Empty(t, one)
	two, _ = GlobalCache.GetByValue("two")
	assert.Empty(t, two)
	three, _ = GlobalCache.GetByValue("three")
	assert.Empty(t, three)
}

func TestCacheTest(t *testing.T) {
	cache := NewCache(time.Millisecond*500, 0)

	key := util.RandomString(10)
	val := util.RandomString(10)

	cache.Add(key, val)

	got, _ := cache.GetByKey(key)
	assert.Equal(t, val, got)

	got, _ = cache.GetByValue(val)
	assert.Equal(t, key, got)

	time.Sleep(time.Millisecond * 501)
	cache.GetByKey("nothing")
	// give time to for go routine to expire
	time.Sleep(time.Millisecond * 100)

	v, _ := cache.GetByKey(key)
	assert.Empty(t, v)
	k, _ := cache.GetByValue(val)
	assert.Empty(t, k)

	cache.Add("one", "one")
	cache.Add("two", "two")
	cache.Add("three", "three")

	cache.Clear()
	one, _ := cache.GetByKey("one")
	assert.Empty(t, one)
	two, _ := cache.GetByKey("two")
	assert.Empty(t, two)
	three, _ := cache.GetByKey("three")
	assert.Empty(t, three)
	one, _ = cache.GetByValue("one")
	assert.Empty(t, one)
	two, _ = cache.GetByValue("two")
	assert.Empty(t, two)
	three, _ = cache.GetByValue("three")
	assert.Empty(t, three)
}

func TestCacheSeenKey(t *testing.T) {
	cache := NewCache(time.Millisecond*500, 0)

	cache.Add("key", "val")

	time.Sleep(time.Millisecond * 300)

	// use get to prolong expiration
	cache.GetByKey("key")

	time.Sleep(time.Millisecond * 300)

	// ensure that since we last used the key, we didn't expire it
	val, _ := cache.GetByKey("key")
	assert.Equal(t, "val", val)

	// now wait for the ttl, and add something else to kick off expiration
	time.Sleep(time.Millisecond * 501)
	cache.Add("key2", "val2")
	time.Sleep(time.Millisecond * 100) // give expiration background some time to complete

	// verify expired
	val, _ = cache.GetByKey("key")
	assert.Empty(t, val)
}

func TestCacheSeenValue(t *testing.T) {
	cache := NewCache(time.Millisecond*500, 0)

	cache.Add("key", "val")

	time.Sleep(time.Millisecond * 300)

	// use get to prolong expiration
	cache.GetByValue("val")

	time.Sleep(time.Millisecond * 300)

	// ensure that since we last used the key, we didn't expire it
	key, _ := cache.GetByValue("val")
	assert.Equal(t, "key", key)

	// now wait for the ttl, and add something else to kick off expiration
	time.Sleep(time.Millisecond * 501)
	cache.Add("none", "none")
	time.Sleep(time.Millisecond * 100) // give expiration background some time to complete

	// verify expired
	key, _ = cache.GetByValue("val")
	assert.Empty(t, key)
}

func TestCacheCapacity(t *testing.T) {
	capacity := (rand.Int() % 100) + 1
	cache := NewCache(time.Millisecond*500, capacity)

	cache.Add("key", "val")

	time.Sleep(time.Millisecond * 501)

	cache.Add(util.RandomString(10), util.RandomString(10))

	val, _ := cache.GetByKey("key")
	assert.Equal(t, "val", val)

	for i := 0; i < capacity; i++ {
		cache.Add(util.RandomString(10), util.RandomString(10))
	}

	time.Sleep(time.Millisecond * 501) // Wait for expire ttl

	cache.Add(util.RandomString(10), util.RandomString(10))

	time.Sleep(time.Millisecond * 100) // Need to wait for go expiring go routine to finish

	val, _ = cache.GetByKey("key")
	assert.Empty(t, val)

	key, _ := cache.GetByValue("val")
	assert.Empty(t, key)
}
