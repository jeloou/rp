# rp (redis proxy)

**rp** is  a fast and lightweight HTTP proxy for [redis](http://redis.io/) protocol. It was built to primarily to reduce the number of requests to a redis server using a local cache.

## Building

To build `rp` you must have a working Go install and [govendor](https://github.com/kardianos/govendor) then you can run:

``` bash
$ govendor build .
```

The suite of tests can be run like this:

``` bash
make test
```

Requires `Make`, `Docker` and `docker-compose`.

```bash
$ rp --help

NAME:
   rp - A fast, light-weight HTTP proxy for redis

USAGE:
   rp [global options] command [command options] [arguments...]

VERSION:
   0.0.0

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --debug                           enable debug output for the logs
   --key-expiry value, -k value      set a global expiry for keys stored in cache (default: "5s")
   --cache-capacity value, -c value  max numer of keys that will be kept in cache (default: 15000)
   --redis-host value                domain of the redis host
   --redis-port value                port of the redis host
   --workers value, -w value         max number of workers to process requests (default: 1)
   --concurrency value, -C value     max number of concurrent clients (default: 30)
   --shutdown-timeout value          set the server max timeout to gracefully shutdown (default: "2s")
   --port value, -P value            HTTP server port (default: "3000")
   --daemonize, -d                   run as a daemon
   --pid value, -p value             set pid file
   --help, -h                        show help
   --version, -v                     print the version
```

## Features

+ Fast.
+ Lightweight.
+ Supports multiple concurrent clients.
+ Limits the number of concurrent connections.
+ Supports LRU caching and non-blocking reads.
+ Cache can be configured to have a global expiry.
+ Keeps multiple connections to the redis server.

## Overview

*rp* consists of a `dispatcher`, `worker` and `cache`.

A request is represented internally as a `job`. A `job` contains the `key` requested by the client and a channel that is used to send responses back to the HTTP handler.

The `dispatcher` handles the incoming HTTP requests using a server running on a local port. That server receives and processes incoming requests, once the request is transformed into a `job` it is ready to be send to the `jobs` channel, a queue that the `dispatcher` uses to assign `job`s to `worker`s. In order to send a `job` to a worker, a available worker must be allocated. The `dispatcher` does that by starting a goroutine that awaits for available `worker`s in a channel. The HTTP handler waits for the `job` to be completed by checking the `res` channel.

There is a fixed number of `workers`. A `worker` makes itself avialable to the `dispatcher` by sending its `job` channel to the `workers` queue, listening to the `jobs` channel until a `job` arrives. Once a `job` is available the worker runs the following tasks:

+ checks if the `key` is available in the `cache`, sends a response back if true.
+ checks if the `key` is available in the `redis` server, sends a response back and saves the response into the `cache`.
+ if the `key` is not found in the `redis` server, an empty response with a `404` code is returned.

The `worker` interacts with the `cache` using two methods: `get` and `set`. Both methods are non-blocking. The `cache` accomplishes this by using a `sync.RWMutex` and a gorouting to process writes. Multiple workers can read from the `cache` at the same time without blocking each other, but only a single worker can write. That way we avoid blocking the `worker`s while they wait for the write lock to be released.

When a `worker` reads a `key` from the `cache`, the `set` method reads the content from a map protected by a `RLock` call. After getting the `key`'s assiciated data, the worker writes the recently created `entity` to the `cache` worker channel and returns the data inmediatly. If the `key` is expired, it returns an empty `string` and writes the `entity` to the queue to be deleted. Setting a new `key`, sends the `key` and `value` to the `cache` worker without blocking.

The `writer` takes care of receiving the `entity` instances and writing to them to the `cache`. 

The `cache` is implemented using a `map` and a doubly-liked list. The `map` gives us fast access to the contents of the `cache` and the list keeps the records ordered by the Least Recently Used (LRU). Reading a `key` from the `cache` means moving the `entity` to the front of the list. But the list does not grow forever, when the max capacity of the `cache` is reached, the last item of the list is removed to make space for the new item.

Errors are bubbled up using an `error`s channel. If an error is triggered by any of the upstream workers or a signal is receive from the OS, *rp* tries to shut down the server gracefully using a context. Failing to do some in a timely manner triggers the forceful shutdown.

It can run as a daemon.

## Complexity

The `cache` implementation has an expected O(1) average time complexity for all operations, but a worst case O(n). It uses a `map` to insert and lookup `keys` and a doubly-liked list to keep the things in order. Doubly-liked lists have a O(1) average time complexity for insertion and deletion.

## License

MIT
