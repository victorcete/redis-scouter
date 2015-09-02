package main

import (
	"expvar"
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
func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle:     8,
		IdleTimeout: 60 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ":6379")
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

func queueStats() {
	// connect to redis
	c := pool.Get()
	c.Send("PSUBSCRIBE", "__keyspace*")
	c.Flush()
	c.Receive()

	// fetch messages
	for {
		reply, err := redis.StringMap(c.Receive())
		if err != nil {
			log.Println(err)

			// Retry connection to Redis until it is back
			defer c.Close()
			log.Printf("connection to redis lost. retry in 1s\n")
			time.Sleep(time.Second * 1)
			c = pool.Get()
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
				Stats.Add(fmt.Sprintf("%s.%s", queue[1], operation), 1)
			}
		}
	}
}

var pool *redis.Pool
var Stats = expvar.NewMap("stats").Init()
var listOperationsRegex = regexp.MustCompile("^(lpush|lpushx|rpush|rpushx|lpop|blpop|rpop|brpop)$")
var keyspaceRegex = regexp.MustCompile("^__keyspace.*__:(?P<queue_name>.*)$")

func main() {
	pool = newPool()
	ticker := time.NewTicker(time.Minute * 1).C
	g := graphite.NewGraphiteNop("localhost", 2003)
	hostname := HostnameGraphite()
	go queueStats()
	for {
		select {
		case <-ticker:
			Stats.Do(func(kv expvar.KeyValue) {
				g.SimpleSend(fmt.Sprintf("temp.%s.%s", hostname, kv.Key), kv.Value.String())
			})
		}
	}
}
