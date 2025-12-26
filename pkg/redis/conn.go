package redis

import (
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
)

var (
	pool      *redis.Pool
	redisHost = "127.0.0.1:6379"
	redisPass = "testupload"
)

func newRedisPool() *redis.Pool {
	return &redis.Pool{
		// Maximum number of idle connections in the pool.
		MaxIdle: 50,
		// Maximum number of connections allocated by the pool at a given time.
		// When zero, there is no limit on the number of connections in the pool.
		MaxActive:   50,
		IdleTimeout: 300 * time.Second,
		Dial: func() (conn redis.Conn, err error) {
			conn, err = redis.Dial("tcp", redisHost)
			if err != nil {
				return nil, err
			}

			if _, err = conn.Do("AUTH", redisPass); err != nil {
				fmt.Println(err)
				conn.Close()
				return nil, err
			}

			return conn, nil

		},
		TestOnBorrow: func(conn redis.Conn, lastUsed time.Time) error {
			if time.Since(lastUsed) > time.Minute {
				_, err := conn.Do("PING")
				if err != nil {
					return err
				}
			}
			return nil

		},
	}
}

func init() {
	pool = newRedisPool()
	data, err := pool.Get().Do("KEYS", "*")
	if err != nil {
		fmt.Println("pool get keys error:%w", err)
	}
	fmt.Println(data)

}

func GetRedisConnectionPool() *redis.Pool {
	return pool
}
