# redis-scouter
Dynamic monitoring for Redis (as a queue)

## Why?
If you use Redis as a queue (using the [LIST](http://redis.io/commands#list) commands) you could end up having zero visibility about what is going on with your queues. If your application workers are not logging any stats at all, you can use redis-scouter as a replacement for it.

The [scouter](http://dragonball.wikia.com/wiki/Scouter) will subscribe to the LIST-related [keyspace notifications](http://redis.io/topics/notifications) and send the statistics to Graphite.

## Prerequisites
Your Redis server must have the keyspace notifications enabled.

You can do that on your redis configuration file adding:
```
notify-keyspace-events Kl
```

or directly on the redis-cli:
```
redis-cli config set notify-keyspace-events Kl
```
