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
	masterCheckInterval int = 5
)

func keyspaceEnable(pool *redis.Pool, port string) {
	c := pool.Get()
	defer c.Close()

	// check if notify-keyspace-events are enabled
	notify, err := redis.StringMap(c.Do("CONFIG", "GET", "notify-keyspace-events"))
	if err != nil {
		log.Printf("[keyspace-check] %s\n", err)
		return
	}

	for _, v := range notify {
		if keyspaceConfigRegex.FindString(v) != "" {
			// already enabled, we can already listen for LIST events
			log.Printf("[keyspace-check-%s] LIST keyspace-notifications already enabled\n", port)
		} else {
			// we need to enable notify-keyspace-events for LIST operations (also do not override previous config if any)
			if v == "" {
				// no previous config was set
				log.Printf("[keyspace-check-%s] Enabling LIST keyspace-notifications (no previous config found)\n", port)
				_, err := redis.String(c.Do("CONFIG", "SET", "notify-keyspace-events", "lK"))
				if err != nil {
					log.Println(err)
					return
				}
			} else {
				// previous config found, do not override
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
			log.Println(err)
			// Retry connection to Redis until it is back
			defer c.Close()
			time.Sleep(time.Second * 1)
			c = pool.Get()
			continue
		}
		for _, value := range master {
			if value != "" {
				// instance is now a slave, notify
				log.Printf("[%s] master-check: instance is now a slave\n", port)
				fetchPossible[port] = false
				//c.Do("PUBLISH", "redis-scouter", "stop")
			} else {
				log.Printf("[%s] master-check: instance is a master\n", port)
				fetchPossible[port] = true
				//c.Do("PUBLISH", "redis-scouter", "start")
			}
		}
		time.Sleep(time.Second * time.Duration(masterCheckInterval))
	}
}

func queueStats(port string) {
	// connect to redis
	pool := newPool(port)
	c := pool.Get()
	if !instanceAlive(pool) {
		log.Printf("error: no redis instance listening on port %s, aborting\n", port)
		return
	}

	// subscribe to the keyspace notifications
	c.Send("PSUBSCRIBE", "__keyspace@*", "redis-scouter")
	c.Flush()
	// ignore first message received when subscribing
	c.Receive()
	c.Receive()

	go instanceIsMaster(pool, port)
	go keyspaceEnable(pool, port)

	for {
		if fetchPossible[port] {
			reply, err := redis.StringMap(c.Receive())
			if err != nil {
				// Retry connection to Redis until it is back
				defer c.Close()
				log.Printf("connection to redis lost. retry in 1s\n")
				time.Sleep(time.Second * 1)
				c = pool.Get()
				go keyspaceEnable(pool, port)
				c.Send("PSUBSCRIBE", "__keyspace@*", "redis-scouter")
				c.Flush()
				c.Receive()
				c.Receive()
				continue
			}
			// match for a LIST keyspace event
			for k, v := range reply {
				operation := listOperationsRegex.FindString(v)
				queue := keyspaceRegex.FindStringSubmatch(k)
				if len(queue) == 2 && operation != "" {
					Stats.Add(fmt.Sprintf("%s.%s.%s", port, queue[1], operation), 1)
				}
			}
		} else {
			log.Println("not fetching stats")
			time.Sleep(time.Second * 1)
		}
	}
}

var Stats = expvar.NewMap("stats").Init()
var listOperationsRegex = regexp.MustCompile("^(lpush|lpushx|rpush|rpushx|lpop|blpop|rpop|brpop)$")
var keyspaceRegex = regexp.MustCompile("^__keyspace.*__:(?P<queue_name>.*)$")
var keyspaceConfigRegex = regexp.MustCompile("^(AK.*|.*l.*K.*)$")
var ports redisPorts
var graph *graphite.Graphite
var fetchPossible = make(map[string]bool)

//var chans = make(map[string](chan bool))

func main() {
	flag.Var(&ports, "ports", "comma-separated list of redis ports")
	graphiteHost := flag.String("graphite-host", "localhost", "graphite hostname")
	graphitePort := flag.Int("graphite-port", 2003, "graphite port")
	interval := flag.Int("interval", 60, "interval for sending graphite metrics")
	simulate := flag.Bool("simulate", false, "simulate sending to graphite via stdout")
	profile := flag.Bool("profile", false, "enable pprof features for cpu/heap/goroutine")
	flag.Parse()

	// flag checks
	if len(ports) == 0 {
		log.Println("no redis instances defined, aborting")
		return
	}

	// simulate graphite sending via stdout
	if *simulate {
		graph = graphite.NewGraphiteNop(*graphiteHost, *graphitePort)
	} else {
		var err error
		graph, err = graphite.NewGraphite(*graphiteHost, *graphitePort)
		if err != nil {
			log.Println(err)
			return
		}
	}

	// check for enabled profiling flag
	if *profile {
		go http.ListenAndServe(":8888", nil)
	}

	hostname := hostnameGraphite()
	ticker := time.NewTicker(time.Second * time.Duration(*interval)).C

	for _, port := range ports {
		fetchPossible[port] = true
		//chans[port] = make(chan bool)
		log.Printf("[instance-%s] starting collector\n", port)
		go queueStats(port)
	}

	sig := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

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
