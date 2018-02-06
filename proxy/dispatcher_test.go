package proxy

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/stretchr/testify/suite"
)

type SuiteHTTPDispatcher struct {
	suite.Suite

	ts *httptest.Server
	c  *redis.Client
	d  *Dispatcher
}

func (s *SuiteHTTPDispatcher) SetupSuite() {
	redisHost := "localhost"
	redisPort := "6379"

	if rh := os.Getenv("REDIS_HOST"); rh != "" {
		redisHost = rh
	}

	if rp := os.Getenv("REDIS_PORT"); rp != "" {
		redisPort = rp
	}

	redisAddr := net.JoinHostPort(redisHost, redisPort)
	s.c = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	_, err := s.c.Ping().Result()
	if err != nil {
		s.FailNow("error connecting to redis")
	}

	err = s.c.Set("k00", "v00", 0).Err()
	if err != nil {
		s.FailNow("error setting up redis")
	}

	// setting up Dispatcher instance
	exp := time.Duration(time.Millisecond * 100)
	maxWorkers := 1
	maxJobs := 1

	ctx, cancel := context.WithCancel(context.Background())
	wCtx, wCancel := context.WithCancel(context.Background())
	cache := newCache(cacheCap, exp, maxWorkers)
	workers := make(chan chan Job)
	jobs := make(chan Job, maxJobs)

	s.d = &Dispatcher{
		redisAddr: redisAddr,

		cache:      cache,
		maxWorkers: maxWorkers,
		workers:    workers,
		jobs:       jobs,

		wCtx:    wCtx,
		wCancel: wCancel,
		ctx:     ctx,
		cancel:  cancel,
	}

	for i := 0; i < maxWorkers; i++ {
		w, err := newWorker(redisAddr, cache, workers)
		if err != nil {
			s.FailNow("error starting worker", err)
		}

		go w.run(wCtx)
	}

	s.ts = httptest.NewServer(httpHandler(s.d))
	go s.d.dispatch()
}

func (s *SuiteHTTPDispatcher) getRequestURI(key string) string {
	u, err := url.Parse(s.ts.URL)
	if err != nil {
		s.FailNow("error parsing URL", err)
	}

	q := u.Query()
	q.Set("key", key)
	u.RawQuery = q.Encode()

	return u.String()
}

func (s *SuiteHTTPDispatcher) TestExistingKey() {
	res, err := http.Get(s.getRequestURI("k00"))
	if err != nil {
		s.FailNow("error making request", err)

	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	s.Equal(http.StatusOK, res.StatusCode, "should be 200")
	s.Equal("v00", string(body), "HTTP response should match value")
}

func (s *SuiteHTTPDispatcher) TestMissingKey() {
	res, err := http.Get(s.getRequestURI("k01"))
	if err != nil {
		s.FailNow("error making request", err)

	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()

	s.Equal(http.StatusNotFound, res.StatusCode, "should be 400")
	s.Equal("", string(body), "HTTP response should be empty")
}

func (s *SuiteHTTPDispatcher) TearDownSuite() {
	err := s.c.Del("k01").Err()
	if err != nil {
		s.FailNow("error tearing down suite")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	s.d.Shutdown(ctx)
	s.ts.Close()
}

func TestHTTPDispatcherSuite(t *testing.T) {
	suite.Run(t, new(SuiteHTTPDispatcher))
}

// redisServer tests
type SuiteRedisDispatcher struct {
	suite.Suite

	ts *httptest.Server
	rs *redisServer
	sc *redis.Client
	c  *redis.Client
	d  *Dispatcher
}

func (s *SuiteRedisDispatcher) SetupSuite() {
	redisHost := "localhost"
	redisPort := "6379"

	redisAddr := net.JoinHostPort(redisHost, redisPort)
	s.sc = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if rh := os.Getenv("REDIS_HOST"); rh != "" {
		redisHost = rh
	}

	if rp := os.Getenv("REDIS_PORT"); rp != "" {
		redisPort = rp
	}

	redisAddr = net.JoinHostPort(redisHost, redisPort)
	s.c = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	_, err := s.c.Ping().Result()
	if err != nil {
		s.FailNow("error connecting to redis")
	}

	err = s.c.Set("k00", "v00", 0).Err()
	if err != nil {
		s.FailNow("error setting up redis")
	}

	// setting up Dispatcher instance
	exp := time.Duration(time.Millisecond * 100)
	maxJobs := 1

	ctx, cancel := context.WithCancel(context.Background())
	wCtx, wCancel := context.WithCancel(context.Background())
	cache := newCache(cacheCap, exp, maxWorkers)
	workers := make(chan chan Job)
	jobs := make(chan Job, maxJobs)

	s.d = &Dispatcher{
		redisAddr: redisAddr,

		cache:      cache,
		maxWorkers: maxWorkers,
		workers:    workers,
		jobs:       jobs,

		wCtx:    wCtx,
		wCancel: wCancel,
		ctx:     ctx,
		cancel:  cancel,
	}

	// setting up worker
	w, err := newWorker(redisAddr, cache, workers)
	if err != nil {
		s.FailNow("error starting worker", err)
	}

	go w.run(wCtx)

	s.ts = httptest.NewServer(httpHandler(s.d))
	go s.d.dispatch()
}

func (s *SuiteRedisDispatcher) TestExistingKey() {
	res, err := s.sc.Get("k00").Result()
	if err != nil {
		s.FailNow("error making request")
	}

	s.Equal("v00", res, "redis response should match value")
}

func (s *SuiteRedisDispatcher) TestMissingKey() {
	_, err := s.sc.Get("k01").Result()
	s.NotEqual(nil, err, "redis response should be an error")
}

func (s *SuiteRedisDispatcher) TearDownSuite() {
	err := s.c.Del("k01").Err()
	if err != nil {
		s.FailNow("error tearing down suite")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	s.d.Shutdown(ctx)
	s.ts.Close()
}

func TestRedisDispatcherSuite(t *testing.T) {
	suite.Run(t, new(SuiteRedisDispatcher))
}
