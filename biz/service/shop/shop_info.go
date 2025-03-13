package shop

import (
	"context"
	"encoding/json"
	"fmt"

	"xzdp/biz/dal/redis"
	"xzdp/biz/pkg/constants"

	shop "xzdp/biz/model/shop"

	"xzdp/biz/dal/mysql"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type ShopInfoService struct {
	RequestContext *app.RequestContext
	Context        context.Context
}

func NewShopInfoService(Context context.Context, RequestContext *app.RequestContext) *ShopInfoService {
	return &ShopInfoService{RequestContext: RequestContext, Context: Context}
}

func (h *ShopInfoService) Run(id int64) (resp *shop.Shop, err error) {
    // 构建 Redis 缓存的键
    key := fmt.Sprintf("%s%d", constants.CACHE_SHOP_KEY, id)

    // 添加日志，记录缓存键
    hlog.Debugf("Trying to get shop data from Redis, key: %s", key)

    // 从 Redis 获取缓存
    result, err := redis.GetStringLogical(h.Context, key, constants.CACHE_SHOP_TTL, WrappedQueryByID, h.Context, id)
    if err != nil {
        hlog.Errorf("Error getting shop data from Redis: %v", err)
        return nil, err
    }

    if result == "" {
        hlog.Infof("Shop data not found in cache, querying from database...")
    } else {
        hlog.Infof("Shop data found in cache, proceeding to unmarshal data.")
    }

    // 解析缓存数据
    err = json.Unmarshal([]byte(result), &resp)
    if err != nil {
        hlog.Errorf("Error unmarshalling shop data from Redis: %v", err)
        return nil, err
    }

    // 返回结果
    hlog.Debugf("Successfully retrieved shop data: %+v", resp)
    return resp, nil
}

func WrappedQueryByID(args ...interface{}) (interface{}, error) {
    // 检查参数
    if len(args) != 2 {
        err := fmt.Errorf("incorrect number of arguments, expected 2, got %d", len(args))
        hlog.Errorf("WrappedQueryByID error: %v", err)
        return nil, err
    }

    // 获取 context 和 id 参数
    ctx, ok := args[0].(context.Context)
    if !ok {
        err := fmt.Errorf("first argument is not context.Context")
        hlog.Errorf("WrappedQueryByID error: %v", err)
        return nil, err
    }

    id, ok := args[1].(int64)
    if !ok {
        err := fmt.Errorf("second argument is not int64")
        hlog.Errorf("WrappedQueryByID error: %v", err)
        return nil, err
    }

    // 添加日志，记录查询的 ID
    hlog.Debugf("Querying shop data from MySQL for ID: %d", id)

    // 调用数据库查询
    result, err := mysql.QueryByID(ctx, id)
    if err != nil {
        hlog.Errorf("Error querying shop data from MySQL: %v", err)
        return nil, err
    }

    // 如果没有数据
    if result == nil {
        hlog.Infof("No shop found for ID: %d", id)
    } else {
        hlog.Debugf("Successfully retrieved shop data from MySQL: %+v", result)
    }

    return result, err
}
