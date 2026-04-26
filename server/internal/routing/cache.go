package routing

import (
	"container/list"
	"sync"
)

// routeCache is a tiny thread-safe LRU keyed by route id. Entries hold
// the decoded LatLon shape points for GPX/KML export plus optional
// summary metadata; the full Valhalla response is NOT retained to keep
// memory predictable (only what the exporters need).
type routeCache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	items    map[string]*list.Element
}

// CachedRoute is what we persist per id. Keep this small — it's held in
// memory for up to `capacity` concurrent routes.
type CachedRoute struct {
	ID             string
	Mode           string
	Shape          []LatLon
	TimeSeconds    float64
	DistanceMeters float64
}

type cacheEntry struct {
	id    string
	value CachedRoute
}

func newRouteCache(capacity int) *routeCache {
	if capacity <= 0 {
		capacity = 256
	}
	return &routeCache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element, capacity),
	}
}

func (c *routeCache) Put(id string, v CachedRoute) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[id]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*cacheEntry).value = v
		return
	}
	el := c.ll.PushFront(&cacheEntry{id: id, value: v})
	c.items[id] = el
	if c.ll.Len() > c.capacity {
		tail := c.ll.Back()
		if tail != nil {
			c.ll.Remove(tail)
			delete(c.items, tail.Value.(*cacheEntry).id)
		}
	}
}

func (c *routeCache) Get(id string) (CachedRoute, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[id]
	if !ok {
		return CachedRoute{}, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).value, true
}
