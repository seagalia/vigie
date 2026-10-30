package services

import (
	"sync"
	"time"
)

// Cache reuses the answer of a source for a ttl, so the flock agents polling together don't each reach the registry
type Cache struct {
	source Service
	ttl    time.Duration
	now    func() time.Time

	// held during the lookup too: requests arriving on an expired answer wait for one lookup instead of all doing it
	mu      sync.Mutex
	ips     []IpAddr
	expires time.Time
}

func NewCache(source Service, ttl time.Duration) *Cache {
	return &Cache{source: source, ttl: ttl, now: time.Now}
}

func (c *Cache) Resolve() ([]IpAddr, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.now().Before(c.expires) {
		return c.ips, nil
	}
	ips, err := c.source.Resolve()
	// not kept, so a registry back up after an outage is seen on the very next request
	if err != nil {
		return nil, err
	}
	c.ips = ips
	c.expires = c.now().Add(c.ttl)
	return ips, nil
}
