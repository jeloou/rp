package proxy

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type stringCmdMock struct {
	mock.Mock
}

func (m *stringCmdMock) Result() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

type redisFetcherMock struct {
	mock.Mock
}

func (m *redisFetcherMock) Get(key string) stringCmd {
	args := m.Called(key)
	return args.Get(0).(stringCmd)
}

func (m *redisFetcherMock) Close() error {
	args := m.Called()
	return args.Error(0)
}

// SuiteWorker
type SuiteWorker struct {
	suite.Suite

	ctx    context.Context
	cancel context.CancelFunc
	rf     redisFetcher
	ws     chan chan Job
	w      *worker
	c      *cache
}

func (s *SuiteWorker) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.c = newCache(cacheCap, defaultExp, maxWorkers)
	s.ws = make(chan chan Job)

	s.w = &worker{
		jobs:    make(chan Job),
		workers: s.ws,
		cache:   s.c,
	}
}

func (s *SuiteWorker) TearDownTestSuite() {
	s.cancel()
	close(s.ws)
}

func (s *SuiteWorker) BeforeTest(suite, test string) {
	if test == "TestCachedRun" {
		s.c.set("k00", "v00")
		<-time.After(time.Millisecond * 5)
		return
	}

	if test == "TestRun" {
		scSuccess := new(stringCmdMock)
		scSuccess.On("Result").Return("v00", nil)
		scSuccess.On("Close").Return(nil)

		scError := new(stringCmdMock)
		scError.On("Result").Return("", errors.New("redis: nil"))
		scError.On("Close").Return(nil)

		rf := new(redisFetcherMock)
		rf.On("Get", "k00").Return(scSuccess)
		rf.On("Get", "k01").Return(scError)

		s.w.client = rf
		return
	}
}

func (s *SuiteWorker) TestRun() {
	go s.w.run(s.ctx)

	res := make(chan *response)
	w := <-s.ws
	w <- Job{
		key: "k00",
		res: res,
	}

	r := <-res
	s.Equal(http.StatusOK, r.code, "should be 200")
	s.Equal("v00", r.body, "redis response should match value")

	w = <-s.ws
	w <- Job{
		key: "k01",
		res: res,
	}

	r = <-res
	s.Equal(http.StatusNotFound, r.code, "should be 400")
	s.Equal("", r.body, "redis response should match value")
}

func (s *SuiteWorker) TestCachedRun() {
	go s.w.run(s.ctx)
	w := <-s.ws

	res := make(chan *response)
	w <- Job{
		key: "k00",
		res: res,
	}

	r := <-res

	s.Equal(http.StatusOK, r.code, "should be 200")
	s.Equal("v00", r.body, "cache response should match value")
}

func TestWorkerSuite(t *testing.T) {
	suite.Run(t, new(SuiteWorker))
}
