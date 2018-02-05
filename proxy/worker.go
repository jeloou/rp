package proxy

import (
	"context"
	"net/http"

	"github.com/go-redis/redis"
	log "github.com/sirupsen/logrus"
)

type response struct {
	code int
	body string
}

type Job struct {
	res chan *response
	key string
}

type worker struct {
	client  *redis.Client
	cache   *cache

	workers chan chan Job
	jobs    chan Job
}

func (w *worker) run(ctx context.Context) {
	defer func() {
		w.client.Close()
	}()

	for {
		w.workers <- w.jobs

		select {
		case <-ctx.Done():
			return
		case job := <-w.jobs:
			v := w.cache.get(job.key)
			if v != "" {
				job.res <- &response{
					code: http.StatusOK,
					body: v,
				}
				continue
			}

			v, err := w.client.Get(job.key).Result()
			if err != nil {
				log.WithFields(log.Fields{
					"key": job.key,
					"error": err,
				}).Error("error while querying redis")

				
				job.res <- &response{
					code: http.StatusNotFound,
				}
				continue
			}

			log.WithFields(log.Fields{
				"key": job.key,
				"value": v,
			}).Debug("key fetched from redis")

			w.cache.set(job.key, v)
			job.res <- &response{
				code: http.StatusOK,
				body: v,
			}
		}
	}
}

func newWorker(redisAddr string, cache *cache, workers chan chan Job) (*worker, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
	})

	return &worker{
		jobs:    make(chan Job),
		workers: workers,
		client:  client,
		cache:   cache,
	}, nil
}
