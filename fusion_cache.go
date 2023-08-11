package GoFusionCache

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
)

var (
	ErrMemoryNotFound = errors.New("memory not found")
	ErrRedisNotFound  = errors.New("redis not found")
)

type Cache[K comparable, V any] interface {
	GetItem(context.Context, K) (V, error)
	SetItem(context.Context, K, V) error
}

type MemoryCache[K comparable, V any] interface {
	Cache[K, V]
}

type RedisCache[K comparable, V any] interface {
	Cache[K, V]
}

type FusionCache[K comparable, V any] struct {
	mc MemoryCache[K, V]
	rc RedisCache[K, V]
}

type Loader[K comparable, V any] func(K) (V, error)

func New[K comparable, V any](mc MemoryCache[K, V], rc RedisCache[K, V]) *FusionCache[K, V] {
	return &FusionCache[K, V]{mc: mc, rc: rc}
}

func (fc *FusionCache[K, V]) Get(c context.Context, k K, loaders ...Loader[K, V]) (V, error) {
	v, err := fc.mc.GetItem(c, k)
	if err == nil {
		return v, nil
	}

	if !errors.Is(err, ErrMemoryNotFound) {
		return v, err
	}

	v, err = fc.rc.GetItem(c, k)
	if err != nil && !errors.Is(err, ErrRedisNotFound) {
		return v, err
	}
	if reflect.ValueOf(v).Interface() != nil {
		err = fc.mc.SetItem(c, k, v)
	} else {
		for _, loader := range loaders {
			v, err = loader(k)
			if err != nil {
				return v, err
			}
			err = fc.mc.SetItem(c, k, v)
			if err != nil {
				return v, err
			}
			err = fc.rc.SetItem(c, k, v)
			if err != nil {
				return v, err
			}
		}
	}
	return v, nil
}

func (fc *FusionCache[K, V]) Set(c context.Context, k K, v V) error {
	err := fc.mc.SetItem(c, k, v)
	if err != nil {
		return err
	}
	err = fc.rc.SetItem(c, k, v)
	if err != nil {
		return err
	}
	return nil
}

type DefaultMemoryCacheImpl[K comparable, V any] struct {
	c *gocache.Cache
}

func (d DefaultMemoryCacheImpl[K, V]) GetItem(c context.Context, k string) (string, error) {
	v, ok := d.c.Get(k)
	if !ok {
		return "", ErrMemoryNotFound
	}
	return v.(string), nil
}

func (d DefaultMemoryCacheImpl[K, V]) SetItem(c context.Context, k string, v string) error {
	d.c.Set(k, v, gocache.NoExpiration)
	return nil
}

type DefaultRedisCacheImpl[K comparable, V any] struct {
	r *redis.Client
}

func (d DefaultRedisCacheImpl[K, V]) GetItem(c context.Context, k string) (string, error) {
	v, err := d.r.Get(c, k).Result()
	if err != nil {
		return "", ErrRedisNotFound
	}
	return v, nil
}

func (d DefaultRedisCacheImpl[K, V]) SetItem(c context.Context, k string, v string) error {
	err := d.r.Set(c, k, v, -1).Err()
	if err != nil {
		return err
	}
	return nil
}

func NewDefaultMemoryCache(defaultExpiration, cleanupInterval time.Duration) MemoryCache[string, string] {
	return &DefaultMemoryCacheImpl[string, string]{c: gocache.New(defaultExpiration, cleanupInterval)}
}

func NewDefaultRedisCache(dsn string) RedisCache[string, string] {
	redisOptions, err := redis.ParseURL(dsn)
	if err != nil {
		panic(fmt.Errorf("failed to parse redis url, got error %s", err))
	}
	if redisOptions.MinIdleConns == 0 {
		redisOptions.MinIdleConns = 1
	}
	if redisOptions.PoolSize == 0 {
		redisOptions.PoolSize = 10
	}

	return &DefaultRedisCacheImpl[string, string]{r: redis.NewClient(redisOptions)}
}

func NewDefaultRedisCacheV2(cli *redis.Client) RedisCache[string, string] {
	return &DefaultRedisCacheImpl[string, string]{r: cli}
}

func NewDefaultFusionCache(dsn string, defaultExpiration, cleanupInterval time.Duration) *FusionCache[string, string] {
	return New(NewDefaultMemoryCache(defaultExpiration, cleanupInterval), NewDefaultRedisCache(dsn))
}
