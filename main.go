package main

import (
	"expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/marpaia/graphite-golang"
)

const (
	masterCheckInterval    int = 1
	connectionLostInterval int = 5
	fetchMetricsInterval   int = 1
)

func keyspaceEnable(pool *redis.Pool, port string) {
	c := pool.Get()
	defer c.Close()

	// Check if notify-keyspace-events are enabled.
	notify, err := redis.StringMap(c.Do("CONFIG", "GET", "notify-keyspace-events"))
	if err != nil {
		log.Printf("[keyspace-check] %s\n", err)
		return
	}

	var keyspaceConfigRegex = regexp.MustCompile("^(AK.*|.*l.*K.*)$")

	for _, v := range notify {
		if keyspaceConfigRegex.FindString(v) != "" {
			// Already enabled, we can already listen for LIST events.
			log.Printf("[keyspace-check-%s] LIST keyspace-notifications already enabled\n", port)
		} else {
			// We need to enable notify-keyspace-events for LIST operations (also do not override previous config if any).
			if v == "" {
				// No previous config was set.
				log.Printf("[keyspace-check-%s] Enabling LIST keyspace-notifications (no previous config found)\n", port)
				_, err := redis.String(c.Do("CONFIG", "SET", "notify-keyspace-events", "lK"))
				if err != nil {
					log.Println(err)
					return
				}
			} else {
				// Previous config found, do not override.
				log.Printf("[keyspace-check-%s] Enabling LIST keyspace-notifications (previous config found)\n", port)
				_, err := redis.String(c.Do("CONFIG", "SET", "notify-keyspace-events", v+"lK"))
				if err != nil {
					log.Println(err)
					return
				}
			}
		}
	}
}

func instanceAlive(pool *redis.Pool) bool {
	c := pool.Get()
	defer c.Close()
	_, err := c.Do("PING")
	if err != nil {
		return false
	}
	return true
}

func instanceIsMaster(pool *redis.Pool, port string) {
	c := pool.Get()
	defer c.Close()

	for {
		master, err := redis.StringMap(c.Do("CONFIG", "GET", "slaveof"))
		if err != nil {
			// Retry connection to Redis until it is back.
			//log.Println(err)
			defer c.Close()
			time.Sleep(time.Second * time.Duration(connectionLostInterval))
			c = pool.Get()
			continue
		}
		for _, value := range master {
			if value != "" {
				// Instance is now a slave, notify.
				if fetchPossible[port] {
					c.Do("PUBLISH", "redis-scouter", "stop")
					fetchPossible[port] = false
					log.Printf("[instance-check-%s] became a slave", port)
				}
			} else {
				// Re-enable metrics.
				if !fetchPossible[port] {
					fetchPossible[port] = true
					log.Printf("[instance-check-%s] became a master", port)
				}
			}
		}
		time.Sleep(time.Second * time.Duration(masterCheckInterval))
	}
}

func queueStats(port string) {
	// Connect to redis.
	pool := newPool(port)
	c := pool.Get()
	if !instanceAlive(pool) {
		log.Printf("error: cannot connect to redis on port %s, aborting\n", port)
		return
	}

	// Subscribe to the keyspace notifications.
	c.Send("PSUBSCRIBE", "__keyspace@*", "redis-scouter")
	c.Flush()
	// Ignore first two ACKs when subscribing.
	c.Receive()
	c.Receive()

	go instanceIsMaster(pool, port)
	go keyspaceEnable(pool, port)

	for {
		if fetchPossible[port] {
			reply, err := redis.StringMap(c.Receive())
			if err != nil {
				// Retry connection to Redis until it is back.
				defer c.Close()
				log.Printf("connection to redis lost. retry in %ds\n", connectionLostInterval)
				time.Sleep(time.Second * time.Duration(connectionLostInterval))
				c = pool.Get()
				//go keyspaceEnable(pool, port)
				c.Send("PSUBSCRIBE", "__keyspace@*", "redis-scouter")
				c.Flush()
				c.Receive()
				c.Receive()
				continue
			}
			// Match for a LIST keyspace event.
			for k, v := range reply {
				if v == "stop" {
					// Break loop if we get a message on redis-scouter pubsub.
					continue
				}
				operation := listOperationsRegex.FindString(v)
				queue := keyspaceRegex.FindStringSubmatch(k)
				if len(queue) == 2 && operation != "" {
					Stats.Add(fmt.Sprintf("%s.%s.%s", port, queue[1], operation), 1)
				}
			}
		} else {
			// Do not fetch stats for now.
			time.Sleep(time.Second * time.Duration(fetchMetricsInterval))
		}
	}
}

var Stats = expvar.NewMap("stats").Init()
var listOperationsRegex = regexp.MustCompile("^(lpush|lpushx|rpush|rpushx|lpop|blpop|rpop|brpop)$")
var keyspaceRegex = regexp.MustCompile("^__keyspace.*__:(?P<queue_name>.*)$")

var graph *graphite.Graphite
var fetchPossible = make(map[string]bool)

func main() {
	graphiteHost := flag.String("graphite-host", "localhost", "graphite hostname")
	graphitePort := flag.Int("graphite-port", 2003, "graphite port")
	interval := flag.Int("interval", 60, "interval for sending graphite metrics")
	simulate := flag.Bool("simulate", true, "simulate sending to graphite via stdout")
	profile := flag.Bool("profile", false, "enable pprof features for cpu/heap/goroutine")
	flag.Parse()

	// Simulate graphite sending via stdout.
	if *simulate {
		graph = graphite.NewGraphiteNop(*graphiteHost, *graphitePort)
	} else {
		var err error
		graph, err = graphite.NewGraphite(*graphiteHost, *graphitePort)
		if err != nil {
			log.Printf("[error] cannot connect to graphite on %s:%d\n", *graphiteHost, *graphitePort)
			return
		}
	}

	hostname := hostnameGraphite()
	ticker := time.NewTicker(time.Second * time.Duration(*interval)).C

	// Auto-discover instances running on the current node.
	ports := discoverInstances()
	if len(ports) == 0 {
		log.Println("[error] no redis instances running on this host")
		return
	}

	// Start the metrics collector for every instance found.
	for _, port := range ports {
		fetchPossible[port] = true
		log.Printf("[instance-%s] starting collector\n", port)
		go queueStats(port)
	}

	sig := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Check for enabled profiling flag.
	if *profile {
		go http.ListenAndServe(":8888", nil)
	}

	go func() {
		<-sig
		done <- true
	}()

	for {
		select {
		case <-ticker:
			Stats.Do(func(kv expvar.KeyValue) {
				graph.SimpleSend(fmt.Sprintf("scouter.%s.%s", hostname, kv.Key), kv.Value.String())
			})
		case <-done:
			log.Printf("[main] user aborted execution\n")
			return
		}
	}
}
