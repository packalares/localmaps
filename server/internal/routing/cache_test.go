package routing

import (
	"fmt"
	"testing"
)

func TestRouteCache_PutGet(t *testing.T) {
	c := newRouteCache(3)
	c.Put("a", CachedRoute{ID: "a"})
	c.Put("b", CachedRoute{ID: "b"})
	if cr, ok := c.Get("a"); !ok || cr.ID != "a" {
		t.Fatalf("expected hit for a, got %+v ok=%v", cr, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("miss should return ok=false")
	}
}

func TestRouteCache_EvictsOldest(t *testing.T) {
	c := newRouteCache(2)
	c.Put("a", CachedRoute{ID: "a"})
	c.Put("b", CachedRoute{ID: "b"})
	// Touch a so b becomes LRU.
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should be present")
	}
	c.Put("c", CachedRoute{ID: "c"})
	if _, ok := c.Get("b"); ok {
		t.Error("b should have been evicted")
	}
	if _, ok := c.Get("a"); !ok {
		t.Error("a should still be present (was most-recently-touched)")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("c should be present")
	}
}

func TestRouteCache_CapacityZeroFallsBack(t *testing.T) {
	c := newRouteCache(0)
	// Default capacity is 256; insert 257 items and make sure we still
	// evict exactly one.
	for i := 0; i < 257; i++ {
		c.Put(fmt.Sprintf("k%d", i), CachedRoute{ID: fmt.Sprintf("k%d", i)})
	}
	if _, ok := c.Get("k0"); ok {
		t.Error("k0 should have been evicted at capacity 256")
	}
	if _, ok := c.Get("k256"); !ok {
		t.Error("k256 should still be present")
	}
}
