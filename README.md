# redis-scouter
Dynamic monitoring for Redis instances used as queues.

![alt text](https://raw.githubusercontent.com/victorcete/redis-scouter/master/img/Scouter.png "OMG it's a DBZ scouter!")

## Why?
One of the most common usages of Redis is to make it behave like a [queue](https://en.wikipedia.org/wiki/Queue_(abstract_data_type)). On top of that, you would probably have some producers and consumers working with its contents.

In a perfect world, you would also have statistics from producer/consumer land, but it's not always the case. So you may end up having a relatively big ecosystem of [LIST](http://redis.io/commands#list) keys used as queues but you won't know the amount of work that is behind unless:

You monitor the instance:
```
redis-cli -p XXXX monitor
``` 

or you enable keyspace notifications to look for `LIST` related commands like `RPUSH` or `LPOP`, for example:
```
redis-cli -p XXXX config set notify-keyspace-events lK
```

## The solution

You can use __redis-scouter__ to monitor what `LIST` keys are being used. It will count the number of `LPUSH LPUSHX RPUSH RPUSHX LPOP BLPOP RPOP BRPOP` operations performed and generate metrics for graphite in the following format:

```
scouter.<hostname>.<instance_port>.<queue_name>.<operation_type> <value> <timestamp>
```

## Prerequisites
Your Redis server should have the keyspace notifications enabled, but __redis-scouter__ will take care of that too.

If you already have some keyspace-events enabled, it __won't__ override the existing config. It will add (if needed) the 'l' and 'K' events to be published. More info about keyspace events [here](http://redis.io/topics/notifications)

If you don't have any kind of keyspace-events being generated, it will set its value to 'lK' to be able to gather the required stats.

## Usage examples
Get __redis-scouter__ from you command line with:
```
go get github.com/victorcete/redis-scouter
go install github.com/victorcete/redis-scouter
```

The binary should be on your PATH now, you can run it with the following flags:
```
$ redis-scouter -h
Usage of redis-scouter:
  -graphite-host string
      graphite hostname (default "localhost")
  -graphite-port int
      graphite port (default 2003)
  -interval int
      interval for sending graphite metrics (default 60)
  -ports value
      comma-separated list of redis ports (default [])
  -profile
      enable pprof features for cpu/heap/goroutine
  -simulate
      simulate sending to graphite via stdout
```

For example, let's get a simulation of the `LIST` stats received on a single instance each 10 seconds with:
```
$ redis-scouter -simulate -ports 6379 -interval 10
2015/09/07 15:28:49 [instance-6379] starting collector
2015/09/07 15:28:49 [keyspace-check-6379] LIST keyspace-notifications already enabled
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_1.lpush 14 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_1.rpop 13 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_10.lpush 16 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_10.rpop 13 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_2.lpush 14 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_2.rpop 13 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_3.lpush 17 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_3.rpop 16 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_4.lpush 15 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_4.rpop 11 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_5.lpush 17 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_5.rpop 17 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_6.lpush 18 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_6.rpop 18 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_7.lpush 18 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_7.rpop 18 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_8.lpush 15 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_8.rpop 7 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_9.lpush 21 2015-09-07 15:28:59
2015/09/07 15:28:59 Graphite: scouter.macbook.6379.queue_9.rpop 13 2015-09-07 15:28:59
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_1.lpush 26 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_1.rpop 15 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_10.lpush 36 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_10.rpop 17 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_2.lpush 32 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_2.rpop 16 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_3.lpush 34 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_3.rpop 22 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_4.lpush 24 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_4.rpop 16 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_5.lpush 33 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_5.rpop 24 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_6.lpush 43 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_6.rpop 20 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_7.lpush 38 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_7.rpop 20 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_8.lpush 28 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_8.rpop 12 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_9.lpush 34 2015-09-07 15:29:09
2015/09/07 15:29:09 Graphite: scouter.macbook.6379.queue_9.rpop 18 2015-09-07 15:29:09
^C2015/09/07 15:29:10 [main] user aborted execution
```

Now with two standalone instances:
```
$ redis-scouter -simulate -ports 6379,6380 -interval 10
2015/09/07 15:30:46 [instance-6379] starting collector
2015/09/07 15:30:46 [instance-6380] starting collector
2015/09/07 15:30:46 [keyspace-check-6380] Enabling LIST keyspace-notifications (no previous config found)
2015/09/07 15:30:46 [keyspace-check-6379] LIST keyspace-notifications already enabled
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_1.lpush 43 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_1.rpop 25 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_2.lpush 35 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_2.rpop 45 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_3.lpush 46 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_3.rpop 38 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_4.lpush 39 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6379.queue_4.rpop 49 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_1.lpush 38 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_1.rpop 40 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_2.lpush 41 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_2.rpop 31 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_3.lpush 49 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_3.rpop 30 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_4.lpush 36 2015-09-07 15:30:56
2015/09/07 15:30:56 Graphite: scouter.macbook.6380.queue_4.rpop 33 2015-09-07 15:30:56
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_1.lpush 78 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_1.rpop 73 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_2.lpush 82 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_2.rpop 92 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_3.lpush 79 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_3.rpop 78 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_4.lpush 87 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6379.queue_4.rpop 107 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_1.lpush 80 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_1.rpop 75 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_2.lpush 79 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_2.rpop 61 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_3.lpush 91 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_3.rpop 73 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_4.lpush 77 2015-09-07 15:31:06
2015/09/07 15:31:06 Graphite: scouter.macbook.6380.queue_4.rpop 72 2015-09-07 15:31:06
^C2015/09/07 15:31:07 [main] user aborted execution
```

## Next steps
- Make it work under master/slave setups

In this case we will be probably writing/reading from the master always, and use the slaves
as backups. If a master goes down or there is a master change, __redis-scouter__ should be able
to detect that, stop monitoring the old master and start monitoring the promoted one.