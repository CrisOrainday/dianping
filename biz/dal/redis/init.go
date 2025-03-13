package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"xzdp/biz/model/cache"
	"xzdp/biz/model/shop"
	"xzdp/biz/pkg/constants"
	"xzdp/conf"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
)

var (
	RedisClient   *redis.Client
	RedsyncClient *redsync.Redsync
)

type ArgsFunc func(args ...interface{}) (interface{}, error)

func Init() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     conf.GetConf().Redis.Address,
		Password: conf.GetConf().Redis.Password,
	})
	if err := RedisClient.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}
	pool := goredis.NewPool(RedisClient)
	RedsyncClient = redsync.New(pool)
}

func SetString(ctx context.Context, key string, value interface{}, duration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = RedisClient.Set(ctx, key, jsonData, duration).Err()
	if err != nil {
		return err
	}
	return nil
}

func SetStringLogical(ctx context.Context, key string, value interface{}, duration time.Duration) error {
	stringValue, err := json.Marshal(value)
	cacheData := cache.NewRedisStringData(string(stringValue), time.Now().Add(duration))
	jsonData, err := json.Marshal(*cacheData)
	if err != nil {
		return err
	}

	err = RedisClient.Set(ctx, key, jsonData, 0).Err()
	if err != nil {
		return err
	}

	return nil
}

//func GetString(ctx context.Context, key string, duration time.Duration) (interface{}, error) {
//
//}

func GetStringLogical(ctx context.Context, key string, duration time.Duration, dbFallback ArgsFunc, args ...interface{}) (string, error) {
    hlog.Debugf("Attempting to get shop data from Redis, key: %s", key)

    redisJson, err := RedisClient.Get(ctx, key).Result()
    if err != nil {
        hlog.Errorf("Error getting data from Redis: %v", err)
    }
    
    lock := NewLock(ctx, constants.LOCK_KEY+key, "lock", duration)
    
    // Redis 缓存未命中或不存在
    if redisJson == "" || errors.Is(err, redis.Nil) {
        hlog.Infof("Cache miss or empty data, attempting to query from database for key: %s", key)

        if lock.TryLock() {
            hlog.Debugf("Lock acquired, querying database asynchronously for key: %s", key)
            
            resultChan := make(chan interface{}, 1)
            go func() {
                defer lock.UnLock("lock")
                
                // 从数据库获取数据
                data, err := dbFallback(args...)
                if err != nil {
                    hlog.Errorf("Error querying database for key %s: %v", key, err)
                    return
                }
                
                // 缓存数据到 Redis
                err = SetStringLogical(ctx, key, data, duration)
                if err != nil {
                    hlog.Errorf("Error setting data in Redis for key %s: %v", key, err)
                    return
                }
                
                hlog.Infof("Successfully queried database and updated Redis for key: %s", key)

                // 返回数据库数据到 channel
                resultChan <- data
            }()
            result := <-resultChan
            shopResult := result.(*shop.Shop)
            byteResult, err := json.Marshal(shopResult)
            return string(byteResult), err
            
        } else {
            hlog.Debugf("Lock could not be acquired for key %s", key)
        }
    } else {
        hlog.Infof("Cache hit for key: %s, data found in Redis", key)
        
        var redisData cache.RedisStringData
        if err := json.Unmarshal([]byte(redisJson), &redisData); err != nil {
            hlog.Errorf("Error unmarshalling Redis data for key %s: %v", key, err)
            return "", err
        }
        
        // 如果缓存未过期
        if redisData.ExpiredTime.After(time.Now()) {
            hlog.Infof("Cache data for key %s is not expired, returning cached data", key)
            return redisData.Data, nil
        } else {
            hlog.Infof("Cache data for key %s is expired, querying database...", key)
            
            if lock.TryLock() {
                hlog.Debugf("Lock acquired, querying database asynchronously for key: %s", key)
                
                go func() {
                    defer lock.UnLock("lock")
                    
                    // 从数据库获取数据
                    data, err := dbFallback(args...)
                    if err != nil {
                        hlog.Errorf("Error querying database for key %s: %v", key, err)
                        return
                    }
                    
                    // 缓存数据到 Redis
                    err = SetStringLogical(ctx, key, data, duration)
                    if err != nil {
                        hlog.Errorf("Error setting data in Redis for key %s: %v", key, err)
                        return
                    }
                    
                    hlog.Infof("Successfully queried database and updated Redis for key: %s", key)
                }()
            } else {
                hlog.Debugf("Lock could not be acquired for key %s", key)
            }
            
            return redisData.Data, nil
        }
    }

    hlog.Warnf("Returning empty data for key: %s", key)
    return "", nil
}

