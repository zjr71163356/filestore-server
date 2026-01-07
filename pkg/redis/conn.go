package redispool

import (
	"fmt"
	"time"

	"filestore-server/config"

	"github.com/gomodule/redigo/redis"
)

var (
	pool *redis.Pool
)

func newRedisPool() *redis.Pool {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("load config for redis: %v\n", err)
		return nil
	}

	addr := cfg.Redis.Addr
	password := cfg.Redis.Password

	return &redis.Pool{
		// Maximum number of idle connections in the pool.
		MaxIdle: 50,
		// Maximum number of connections allocated by the pool at a given time.
		// When zero, there is no limit on the number of connections in the pool.
		MaxActive:   50,
		IdleTimeout: 300 * time.Second,
		Dial: func() (conn redis.Conn, err error) {
			conn, err = redis.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}

			if password != "" {
				if _, err = conn.Do("AUTH", password); err != nil {
					fmt.Println(err)
					conn.Close()
					return nil, err
				}
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
	if pool == nil {
		return
	}
	conn := pool.Get()
	if conn == nil {
		fmt.Println("pool get connection is nil")
		return
	}
	defer conn.Close()

	if _, err := conn.Do("PING"); err != nil {
		fmt.Printf("redis ping failed: %v\n", err)
	}

}

func GetRedisConnectionPool() *redis.Pool {
	return pool
}
