package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

const pollingInterval = time.Millisecond * 500

type Dispatcher struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wCtx    context.Context
	wCancel context.CancelFunc
	errs    chan<- error

	redisSrv *redisServer
	srv      *http.Server

	maxWorkers int
	workers    chan chan Job
	jobs       chan Job
	cache      *cache

	redisAddr string
}

func (d *Dispatcher) Run() error {
	client := redis.NewClient(&redis.Options{
		Addr: d.redisAddr,
	})

	_, err := client.Ping().Result()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Error("error while connecting to redis")
		return err
	}

	client.Close()

	for i := 0; i < d.maxWorkers; i++ {
		w, err := newWorker(d.redisAddr, d.cache, d.workers)
		if err != nil {
			return err
		}

		go w.run(d.wCtx)
	}

	log.WithFields(log.Fields{
		"workers": d.maxWorkers,
	}).Debug("pool of workers started")

	d.srv.Handler = httpHandler(d)
	go func() {
		if err := d.srv.ListenAndServe(); err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("error starting server")
			d.errs <- err
		}
	}()

	d.redisSrv.Handler = redisHandler(d)
	go func() {
		if err := d.redisSrv.ListenAndServe(); err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("error starting redis server")
			d.errs <- err
		}
	}()

	d.dispatch()
	return nil
}

func (d *Dispatcher) dispatch() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case job := <-d.jobs:
			go func(job Job) {
				worker := <-d.workers
				worker <- job
			}(job)
		}
	}
}

func redisHandler(d *Dispatcher) func(string) (string, error) {
	return func(key string) (string, error) {
		work := Job{
			res: make(chan *response),
			key: key,
		}

		select {
		case d.jobs <- work:
			res := <-work.res
			if res.code == http.StatusNotFound {
				return "", errors.New("key not found")
			}

			return res.body, nil
		default:
			return "", errors.New("service unavailable")
		}
	}
}

func httpHandler(d *Dispatcher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		key := r.FormValue("key")
		if key == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		work := Job{
			res: make(chan *response),
			key: key,
		}

		select {
		case d.jobs <- work:
			res := <-work.res
			w.WriteHeader(res.code)
			fmt.Fprint(w, res.body)
			return
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	})
}

func (d *Dispatcher) Shutdown(ctx context.Context) error {
	if ctx == nil {
		panic("ctx must be provided")
	}

	d.cancel()

	t := time.NewTicker(pollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("shutdown timeout exceeded, killing workers")
			d.wCancel()
			return ctx.Err()
		case <-t.C:
			log.WithFields(log.Fields{
				"workers left": d.maxWorkers - len(d.workers),
			}).Info("checking if workers are done")

			if len(d.workers) == d.maxWorkers {
				log.Info("all workers done")
				return nil
			}
		}
	}
}

func NewDispatcher(port string, redisAddr string, maxJobs uint, maxWorkers uint, cacheCap int, exp time.Duration, errs chan<- error) (*Dispatcher, error) {
	redisSrv := &redisServer{
		Addr: net.JoinHostPort("", "6380"),
	}

	srv := &http.Server{
		Addr: net.JoinHostPort("", port),
	}

	ctx, cancel := context.WithCancel(context.Background())
	wCtx, wCancel := context.WithCancel(context.Background())

	workers := make(chan chan Job, maxWorkers)
	jobs := make(chan Job, maxJobs)

	return &Dispatcher{
		redisAddr: redisAddr,

		cache:      newCache(cacheCap, exp, int(maxWorkers)),
		maxWorkers: int(maxWorkers),
		workers:    workers,

		jobs: jobs,

		wCtx:    wCtx,
		wCancel: wCancel,
		ctx:     ctx,
		cancel:  cancel,
		errs:    errs,

		redisSrv: redisSrv,
		srv:      srv,
	}, nil
}
