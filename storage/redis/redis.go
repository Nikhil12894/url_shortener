package redis

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/Nikhil12894/url_shortener/base62"
	"github.com/Nikhil12894/url_shortener/storage"
	"github.com/gomodule/redigo/redis"
)

type redisStruct struct{ pool *redis.Pool }

func New(host, port, password, db string) (*redisStruct, error) {
	pool := &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", fmt.Sprintf("%s:%s", host, port))
			if err != nil {
				return nil, err
			}
			if _, err := c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}
			// if _, err := c.Do("SELECT", db); err != nil {
			// 	c.Close()
			// 	return nil, err
			// }
			return c, nil
		},
	}

	return &redisStruct{pool}, nil
}

func (r *redisStruct) isUsed(id uint64) bool {
	conn := r.pool.Get()
	defer conn.Close()

	exists, err := redis.Bool(conn.Do("EXISTS", "Shortener:"+strconv.FormatUint(id, 10)))
	if err != nil {
		return false
	}
	return exists
}

func (r *redisStruct) Save(url string, expires time.Time) (string, error) {
	conn := r.pool.Get()
	defer conn.Close()

	var id uint64

	for used := true; used; used = r.isUsed(id) {
		id = rand.Uint64()
	}

	shortLink := storage.Item{id, url, expires.Format("2006-01-02 15:04:05.728046 +0300 EEST"), 0}

	_, err := conn.Do("HMSET", redis.Args{"Shortener:" + strconv.FormatUint(id, 10)}.AddFlat(shortLink)...)
	if err != nil {
		return "", err
	}

	_, err = conn.Do("EXPIREAT", "Shortener:"+strconv.FormatUint(id, 10), expires.Unix())
	if err != nil {
		return "", err
	}

	return base62.Encode(id), nil
}

func (r *redisStruct) Load(code string) (string, error) {
	conn := r.pool.Get()
	defer conn.Close()

	decodedId, err := base62.Decode(code)
	if err != nil {
		return "", err
	}

	urlString, err := redis.String(conn.Do("HGET", "Shortener:"+strconv.FormatUint(decodedId, 10), "url"))
	if err != nil {
		return "", err
	} else if len(urlString) == 0 {
		return "", storage.ErrNoLink
	}

	_, err = conn.Do("HINCRBY", "Shortener:"+strconv.FormatUint(decodedId, 10), "visits", 1)

	return urlString, nil
}

func (r *redisStruct) isAvailable(id uint64) bool {
	conn := r.pool.Get()
	defer conn.Close()

	exists, err := redis.Bool(conn.Do("EXISTS", "Shortener:"+strconv.FormatUint(id, 10)))
	if err != nil {
		return false
	}
	return !exists
}

func (r *redisStruct) LoadInfo(code string) (*storage.Item, error) {
	conn := r.pool.Get()
	defer conn.Close()

	decodedId, err := base62.Decode(code)
	if err != nil {
		return nil, err
	}

	values, err := redis.Values(conn.Do("HGETALL", "Shortener:"+strconv.FormatUint(decodedId, 10)))
	if err != nil {
		return nil, err
	} else if len(values) == 0 {
		return nil, storage.ErrNoLink
	}
	var shortLink storage.Item
	err = redis.ScanStruct(values, &shortLink)
	if err != nil {
		return nil, err
	}

	return &shortLink, nil
}

func (r *redisStruct) Close() error {
	return r.pool.Close()
}
