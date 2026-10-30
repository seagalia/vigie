package services

import (
	"sync"
	"time"

	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
)

// Cache reuses the answer of a source for a ttl, so the flock agents polling together don't each reach the registry
type Cache struct {
	source Service
	ttl    time.Duration
	now    func() time.Time

	// held during the lookup too: requests arriving on an expired answer wait for one lookup instead of all doing it
	mu        sync.Mutex
	endpoints []*eds.Endpoint
	expires   time.Time
}

func NewCache(source Service, ttl time.Duration) *Cache {
	return &Cache{source: source, ttl: ttl, now: time.Now}
}

func (c *Cache) Resolve() ([]*eds.Endpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.now().Before(c.expires) {
		return c.endpoints, nil
	}
	endpoints, err := c.source.Resolve()
	// not kept, so a registry back up after an outage is seen on the very next request
	if err != nil {
		return nil, err
	}
	c.endpoints = endpoints
	c.expires = c.now().Add(c.ttl)
	return endpoints, nil
}
