// Package cache implements a simple time-outable cache
package cache

import (
	"errors"
	"sync"
	"time"
)

// Cache represents a simple safe
type Cache struct {
	cache  map[string]interface{}
	expire map[string]time.Time
	lock   sync.Mutex
}

var (
	// ErrTimeoutExpired is the error returned along with the value when the timeout
	// for the value has expired
	ErrTimeoutExpired = errors.New("timeout expired, value may be invalid now")
	// ErrWrongType is the error returned when the value could not be typecast
	// into the requested type
	ErrWrongType = errors.New("the value does not conform to the requested type")
)

// NewCache creates a new Cache and returns a pointer to it
func NewCache() *Cache {
	return &Cache{
		cache:  make(map[string]interface{}),
		expire: make(map[string]time.Time),
	}
}

// Write sets the value of the cache to v for key k. It also clears the
// expiration time for the key, if it existed
func (c *Cache) Write(k string, v interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[k] = v
	delete(c.expire, k)
}

// WriteTimeout sets the value of the cache to v for key k. It sets the
// expiration time for the key to now + e
func (c *Cache) WriteTimeout(k string, v interface{}, e time.Duration) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.cache[k] = v
	c.expire[k] = time.Now().Add(e)
}

// ReadCache returns the value of the cache for key k. If the value has
// expeired, it also returns the ErrTimeoutExpired error
func (c *Cache) Read(k string) (interface{}, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	v := c.cache[k]
	e := c.expire[k]
	if !e.IsZero() && time.Now().After(e) {
		return v, ErrTimeoutExpired
	}
	return v, nil
}

// Clear deletes the value of the cache for key k and its associated timeout
func (c *Cache) Clear(k string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.cache, k)
	delete(c.expire, k)
}

// ReadString returns the value of the cache for key k typecast into a string.
// If it cannot be typecast, it will return ErrWrongType. The method will
// return an ErrTimeoutExpired if appropriate and the typecast succeeded
func (c *Cache) ReadString(k string) (string, error) {
	v, e := c.Read(k)
	if v == nil {
		return "", e
	}
	switch v.(type) {
	case string:
		return v.(string), e
	default:
		return "", ErrWrongType
	}
}
