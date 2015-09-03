package main

import (
	"expvar"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/marpaia/graphite-golang"
)

// https://godoc.org/github.com/garyburd/redigo/redis#Pool
func newPool(port string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     4,
		IdleTimeout: 60 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ":"+port)
			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

// fqdn with underscores
func HostnameGraphite() string {
	hostname, _ := os.Hostname()
	return strings.Replace(hostname, ".", "_", -1)
}

func keyspaceEnable(pool *redis.Pool) {
	c := pool.Get()
	defer c.Close()

	// check if notify-keyspace-events are enabled
	notify, err := redis.StringMap(c.Do("CONFIG", "GET", "notify-keyspace-events"))
	if err != nil {
		log.Println(err)
		return
	}
	for _, v := range notify {
		config := keyspaceConfigRegex.FindString(v)
		if config != "" {
			// already enabled, we can listen for the LIST events
			log.Println("LIST events notifications already enabled")
		} else {
			// enable LIST events without replacing the existing config (if any)
			log.Println("Enabling LIST events notifications")
			if v == "" {
				_, err := redis.String(c.Do("CONFIG", "SET", "notify-keyspace-events", "lK"))
				if err != nil {
					log.Println(err)
					return
				}
			} else {
				// do not override the existing config
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

func instanceIsMaster(pool *redis.Pool, port string) bool {
	c := pool.Get()
	defer c.Close()

	master, err := redis.StringMap(c.Do("CONFIG", "GET", "slaveof"))
	if err != nil {
		log.Println(err)
		return false
	}
	for _, value := range master {
		if value == "" {
			log.Printf("instance on port %s is a master\n", port)
			return true
		} else {
			log.Printf("instance on port %s is a slave of %s\n", port, value)
		}
	}
	return false
}

func queueStats(port string) {
	// connect to redis
	pool := newPool(port)
	c := pool.Get()
	if !instanceAlive(pool) {
		log.Printf("error: no redis instance listening on port %s, aborting\n", port)
		return
	}

	go keyspaceEnable(pool)

	// subscribe to the keyspace notifications
	c.Send("PSUBSCRIBE", "__keyspace*")
	c.Flush()
	// ignore first message received when subscribing
	c.Receive()

	// wait for published notifications
	for {
		reply, err := redis.StringMap(c.Receive())
		if err != nil {
			log.Println(err)

			// Retry connection to Redis until it is back
			defer c.Close()
			log.Printf("connection to redis lost. retry in 1s\n")
			time.Sleep(time.Second * 1)
			c = pool.Get()
			go keyspaceEnable(pool)
			c.Send("PSUBSCRIBE", "*")
			c.Flush()
			c.Receive()
			continue
		}
		// match for a LIST keyspace event
		for k, v := range reply {
			operation := listOperationsRegex.FindString(v)
			queue := keyspaceRegex.FindStringSubmatch(k)
			if len(queue) == 2 && operation != "" {
				//log.Printf("%s on %s queue\n", operation, queue[1])
				Stats.Add(fmt.Sprintf("%s.%s.%s", port, queue[1], operation), 1)
			}
		}
	}
}

var Stats = expvar.NewMap("stats").Init()
var listOperationsRegex = regexp.MustCompile("^(lpush|lpushx|rpush|rpushx|lpop|blpop|rpop|brpop)$")
var keyspaceRegex = regexp.MustCompile("^__keyspace.*__:(?P<queue_name>.*)$")
var keyspaceConfigRegex = regexp.MustCompile("^(AK.*|.*l.*K.*)$")
var ports redisPorts
var graph *graphite.Graphite

func main() {
	flag.Var(&ports, "ports", "comma-separated list of redis ports")
	graphiteHost := flag.String("graphite-host", "localhost", "graphite hostname")
	graphitePort := flag.Int("graphite-port", 2003, "graphite port")
	interval := flag.Int("interval", 60, "interval for sending graphite metrics")
	simulate := flag.Bool("simulate", false, "simulate sending to graphite via stdout")
	flag.Parse()

	// flag checks
	if len(ports) == 0 {
		log.Println("no redis instances defined, aborting")
		return
	}

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
	hostname := HostnameGraphite()
	ticker := time.NewTicker(time.Second * time.Duration(*interval)).C

	for _, port := range ports {
		log.Println("spawning collector for port ", port)
		go queueStats(port)
	}

	for {
		select {
		case <-ticker:
			Stats.Do(func(kv expvar.KeyValue) {
				graph.SimpleSend(fmt.Sprintf("scouter.%s.%s", hostname, kv.Key), kv.Value.String())
			})
		}
	}
}
