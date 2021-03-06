package main

import (
	"context"
	"net"
	"time"

	"github.com/jeloou/rp/proxy"
	"github.com/urfave/cli"
)

type command struct {
	dispatcher      *proxy.Dispatcher
	shutdownTimeout time.Duration
}

func (c *command) Run() error {
	err := c.dispatcher.Run()
	if err != nil {
		return err
	}

	return nil
}

func (c *command) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.shutdownTimeout)
	defer cancel()

	return c.dispatcher.Shutdown(ctx)
}

func newCommand(ctx *cli.Context, errs chan<- error) (*command, error) {
	redisServerPort := ctx.GlobalString("redis-server-port")
	redisAddr := net.JoinHostPort(ctx.GlobalString("redis-host"), ctx.GlobalString("redis-port"))
	shutdownTimeout, err := time.ParseDuration(ctx.GlobalString("shutdown-timeout"))
	if err != nil {
		return nil, err
	}

	concurrency := ctx.GlobalUint("concurrency")
	workers := ctx.GlobalUint("workers")
	port := ctx.GlobalString("port")

	cacheCap := ctx.GlobalInt("cache-capacity")
	exp, err := time.ParseDuration(ctx.GlobalString("key-expiry"))
	if err != nil {
		return nil, err
	}

	d, err := proxy.NewDispatcher(port, redisAddr, redisServerPort, concurrency, workers, cacheCap, exp, errs)
	if err != nil {
		return nil, err
	}

	return &command{
		shutdownTimeout: shutdownTimeout,
		dispatcher:      d,
	}, nil
}
