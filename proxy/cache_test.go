package proxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	defaultExp = time.Duration(time.Millisecond * 100)
	maxWorkers = 1
	cacheCap   = 3
)

type SuiteCache struct {
	suite.Suite
	c *cache
}

func (s *SuiteCache) SetupSuite() {
	s.c = newCache(cacheCap, defaultExp, maxWorkers)
}

func (s *SuiteCache) SetupTest() {
	s.c.set("k00", "v00")
	s.c.set("k01", "v01")
	s.c.set("k02", "v02")
	<-time.After(time.Millisecond * 5)
}

func (s *SuiteCache) TestSet() {
	s.c.set("k03", "v03")
	<-time.After(time.Millisecond * 5)

	keys := getCacheKeys(s.c)
	s.Equal(cacheCap, len(keys), "cache should't exeed the max capacity")
	s.Equal([]string{"k03", "k02", "k01"}, keys, "keys should be stored in order")

	s.c.set("k00", "v00")
	<-time.After(time.Millisecond * 5)

	keys = getCacheKeys(s.c)
	s.Equal(cacheCap, len(keys), "cache should't exceed the max capacity")
	s.Equal([]string{"k00", "k03", "k02"}, keys, "keys should be stored in order")
}

func (s *SuiteCache) TestGet() {
	v := s.c.get("k03")
	s.Equal("", v, "shouldn't get value for empty key")

	v = s.c.get("k00")
	s.Equal("v00", v, "should get value for existing key")
}

func getCacheKeys(c *cache) []string {
	l := c.l
	k := []string{}
	for el := l.Front(); el != nil; el = el.Next() {
		e := el.Value.(*entry)
		k = append(k, e.key)
	}

	return k
}

func TestCacheSuite(t *testing.T) {
	suite.Run(t, new(SuiteCache))
}

type SuiteExpCache struct {
	suite.Suite
	c *cache
}

func (s *SuiteExpCache) SetupSuite() {
	exp := time.Duration(time.Millisecond * 10)
	s.c = newCache(cacheCap, exp, maxWorkers)
}

func (s *SuiteExpCache) SetupTest() {
	s.c.set("k00", "v00")
	s.c.set("k01", "v01")
	s.c.set("k02", "v02")
	<-time.After(time.Millisecond * 5)
}

func (s *SuiteExpCache) TestGet() {
	v := s.c.get("k00")
	s.Equal("v00", v, "should get value for existing key")

	<-time.After(10)

	v = s.c.get("k00")
	s.Equal("", v, "shouldn't get value for expired key")
}
