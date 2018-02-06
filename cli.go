package main

import (
	"os"
	"os/signal"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func before(ctx *cli.Context) error {
	if ctx.GlobalBool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	return nil
}

func action(ctx *cli.Context, sigs <-chan os.Signal) error {
	errs := make(chan error)
	defer close(errs)

	c, err := newCommand(ctx, errs)
	if err != nil {
		return err
	}

	go func() {
		err := c.Run()
		if err != nil {
			errs <- err
		}
	}()

	select {
	case <-sigs:
		log.Info("shutdown signal triggered")
	case err := <-errs:
		log.WithFields(log.Fields{
			"error": err,
		}).Error("server error triggered")
	}

	if err := c.Stop(); err != nil {
		log.Error("server failed to gracefully shut down:", err)
		return err
	}

	log.Info("server shutdown")
	return nil
}

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = "rp"
	app.Usage = "A fast, light-weight HTTP proxy for redis"
	app.Before = before
	app.Action = handleSignal(action)

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "enable debug output for the logs",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:  "key-expiry,k",
			Usage: "set a global expiry for keys stored in cache",
			Value: "5s",
		},
		cli.IntFlag{
			Name:  "cache-capacity,c",
			Usage: "max numer of keys that will be kept in cache",
			Value: 15000,
		},
		cli.StringFlag{
			Name:   "redis-host",
			Usage:  "domain of the redis host",
			Value:  "localhost",
			EnvVar: "REDIS_HOST",
		},
		cli.StringFlag{
			Name:   "redis-port",
			Usage:  "port of the redis host",
			Value:  "6379",
			EnvVar: "REDIS_PORT",
		},
		cli.UintFlag{
			Name:  "workers,w",
			Usage: "max number of workers to process requests",
			Value: 1,
		},
		cli.UintFlag{
			Name:  "concurrency,C",
			Usage: "max number of concurrent clients",
			Value: 30,
		},
		cli.StringFlag{
			Name:  "shutdown-timeout",
			Usage: "set the server max timeout to gracefully shutdown",
			Value: "2s",
		},
		cli.StringFlag{
			Name:  "port,P",
			Usage: "HTTP server port",
			Value: "3000",
		},
	}

	return app
}

func handleSignal(fn func(ctx *cli.Context, sigs <-chan os.Signal) error) cli.ActionFunc {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Kill, os.Interrupt)

	return func(ctx *cli.Context) error {
		defer close(sigs)
		return fn(ctx, sigs)
	}
}
