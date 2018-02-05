package proxy

import (
	"time"
	"sync"
	"container/list"

	log "github.com/sirupsen/logrus"
)

type entry struct {
	key string
        val string
        exp time.Time
}

type cache struct {
        m    map[string]*list.Element
        l    *list.List
	exp  time.Duration
	cap  int

        mu sync.RWMutex
	w  *writer
}

func (c *cache) get(k string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	el, ok := c.m[k]
	if !ok {
		log.WithFields(log.Fields{
			"key": k,
		}).Debug("key doesn't exist in cache")
		return ""
	}

	e := el.Value.(*entry)
	if time.Now().After(e.exp) {
		c.w.q <- e
		return ""
	}

	c.w.q <- e
	return e.val
}

func (c *cache) set(k string, v string) {
	exp := time.Now().Add(c.exp)
	c.w.q <- &entry{k, v, exp}
}

type writer struct {
	q  chan *entry
	c  *cache
}

func (w *writer) write(e *entry) {
	c := w.c

	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.m[e.key]
	if !ok {
		if c.l.Len() == c.cap {
			c.l.Remove(c.l.Back())
		}

		el = c.l.PushFront(e)
		c.m[e.key] = el

		log.WithFields(log.Fields{
			"key": e.key,
			"value": e.val,
		}).Debug("new key written into cache")
		return
	}

	log.WithFields(log.Fields{
		"key": e.key,
	}).Debug("key found in cache, moving to front")
	c.l.MoveToFront(el)
}

func (w *writer) del(e *entry) {
	c := w.c

	c.mu.Lock()
	defer c.mu.Unlock()

	el := c.m[e.key]
	c.l.Remove(el)

	delete(c.m, e.key)

	log.WithFields(log.Fields{
		"key": e.key,
	}).Debug("expired key found, deleted from cache")
}

func (w *writer) run() {
	log.Debug("cache writer is running")


	for e := range w.q {
		if time.Now().After(e.exp) {
			w.del(e)
			continue
		}

		w.write(e)
	}
}

func newWriter(c *cache, s int) *writer {
	return &writer{
		q: make(chan *entry, s),
		c: c,
	}
}

func newCache(cap int, exp time.Duration, s int) *cache {
        m := make(map[string]*list.Element)
        l := list.New()

	c := &cache{
		cap: cap,
		exp: exp,
	        m: m,
	        l: l,
        }

	c.w = newWriter(c, s)
	go c.w.run()

	return c
}

