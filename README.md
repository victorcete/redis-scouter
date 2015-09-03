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

You can use redis-scouter to monitor what `LIST` keys are being used. It will count the number of `LPUSH LPUSHX RPUSH RPUSHX LPOP BLPOP RPOP BRPOP` operations performed and generate metrics for graphite in the following format:

```
scouter.<hostname>.<instance_port>.<queue_name>.<operation_type> <value> <timestamp>
```

## Prerequisites
Your Redis server should have the keyspace notifications enabled, but redis-scouter will take care of that too.

If you already have some keyspace-events enabled, it *won't* override the existing config. It will add (if needed) the 'l' and 'K' events to be published.

If you don't have any kind of keyspace-events being generated, it will set its value to 'lK' to be able to gather the required stats.