package utils

import (
	"time"

	"github.com/FZambia/sentinel"
	"github.com/gomodule/redigo/redis"
	log "github.com/inconshreveable/log15"
	"github.com/spf13/viper"
)

/*
func NewPool() *redis.Pool {
	pool := &redis.Pool{
		MaxIdle:     3,
		MaxActive:   5,
		IdleTimeout: 10 * time.Second,
		// Other pool configuration not shown in this example.
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", viper.GetString("REDIS.url"))
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("AUTH", viper.GetString("REDIS.auth")); err != nil {
				c.Close()
				return nil, err
			}
			if _, err := c.Do("SELECT", viper.GetString("REDIS.db")); err != nil {
				c.Close()
				return nil, err
			}
			return c, nil
		},
	}
	return pool
}
*/

func NewPool() *redis.Pool {
	sntnl := &sentinel.Sentinel{
		Addrs:      viper.GetStringSlice("REDIS.url"),
		MasterName: viper.GetString("REDIS.master"),
		Dial: func(addr string) (redis.Conn, error) {
			timeout := 5 * time.Second
			c, err := redis.DialTimeout("tcp", addr, timeout, timeout, timeout)
			if err != nil {
				log.Error("REDIS DIAL", err)
				return nil, err
			}
			return c, nil
		},
	}
	return &redis.Pool{
		MaxIdle:     3,
		MaxActive:   5,
		IdleTimeout: 60 * time.Second,
		Dial: func() (redis.Conn, error) {
			masterAddr, err := sntnl.MasterAddr()
			if err != nil {
				log.Error("SENTINEL MASTER", err)
				return nil, err
			}
			c, err := redis.Dial("tcp", masterAddr)
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("AUTH", viper.GetString("REDIS.auth")); err != nil {
				c.Close()
				return nil, err
			}
			if _, err := c.Do("SELECT", viper.GetString("REDIS.db")); err != nil {
				c.Close()
				return nil, err
			}
			return c, nil
		},
	}
}
