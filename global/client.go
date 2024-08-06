package global

import (
	"sync"
)

var ClientCacher = NewClientCache()

type ClientCache struct {
	ExitsIpMap map[string]bool
	Mu         sync.RWMutex
	IpChan     chan Messages
}

func (c *ClientCache) ClientSet(ip string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ExitsIpMap[ip] = true
}

func (c *ClientCache) ClientDel(ip string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	delete(c.ExitsIpMap, ip)
}

func (c *ClientCache) ClientGet(ip string) bool {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	return c.ExitsIpMap[ip]
}

func (c *ClientCache) GetMap() map[string]bool {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	return c.ExitsIpMap
}

func (c *ClientCache) ClearClientCacheMap() {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	c.ExitsIpMap = make(map[string]bool)
}

func NewClientCache() *ClientCache {
	return &ClientCache{
		ExitsIpMap: make(map[string]bool),
		Mu:         sync.RWMutex{},
		IpChan:     make(chan Messages, 10000),
	}
}
