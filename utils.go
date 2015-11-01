package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
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

// FQDN with underscores.
func hostnameGraphite() string {
	hostname, _ := os.Hostname()
	return strings.Replace(hostname, ".", "_", -1)
}

// Generic method to check if an element exists in a slice.
func contains(s []string, i string) bool {
	for _, v := range s {
		if v == i {
			return true
		}
	}
	return false
}

// Custom type to store the comma-separated list of
// redis ports to be monitored (in case there is more
// than one instance on the host).
type redisPorts []string

// Set implementation for flag.Value interface.
func (i *redisPorts) Set(value string) error {
	if len(*i) > 0 {
		return errors.New("redis ports flag already set")
	}
	for _, v := range strings.Split(value, ",") {
		if !contains(*i, v) {
			*i = append(*i, v)
		}
	}
	return nil
}

// String implementation for flag.Value interface.
func (i *redisPorts) String() string {
	return fmt.Sprint(*i)
}

func discoverInstances() []string {
	log.Printf("[utils] starting auto-discovery\n")
	cmd := "ps -ef | grep [r]edis-server | awk '{print $9}' | cut -d':' -f2"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Printf("[utils] auto-discovery failed: %s\n", err)
		return nil
	}
	found := strings.Split(string(out), "\n")
	// Skip the last newline character present on the array.
	ports := found[:len(found)-1]
	for _, port := range ports {
		log.Printf("[utils] auto-discovery: found redis-server instance on port %s\n", port)
	}
	return ports
}
