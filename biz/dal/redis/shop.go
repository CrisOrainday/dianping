package redis

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"xzdp/biz/model/shop"
	"xzdp/biz/pkg/constants"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/go-redis/redis/v8"
)

func GetShopFromCache(ctx context.Context, key string) (*shop.Shop, error) {
	shopJson, err := RedisClient.Get(ctx, key).Result()
	if err == nil && shopJson != "" {
		var shop shop.Shop
		if err := json.Unmarshal([]byte(shopJson), &shop); err != nil {
			return nil, err
		}
		return &shop, nil
	}

	if err != redis.Nil {
		return nil, err
	}

	if shopJson == "" {
		return nil, errors.New("shop not found in cache")
	}

	return nil, errors.New("unknown error")
}

func QueryShopWithDistance(ctx context.Context, req *shop.ShopOfTypeGeoReq) (resp *[]*shop.Shop, err error) {
    // Log the incoming request parameters
    hlog.Infof("Received request: ShopOfTypeGeoReq{Current: %d, Distance: %f, Longitude: %f, Latitude: %f, TypeId: %d}",
        req.Current, req.Distance, req.Longitude, req.Latitude, req.TypeId)

    current := req.Current
    from := (current - 1) * constants.DEFAULT_PAGE_SIZE
    end := current * constants.DEFAULT_PAGE_SIZE

    key := constants.SHOP_GEO_KEY + strconv.Itoa(int(req.TypeId))

    hlog.Infof("Querying Redis for shops within radius %f of point (Longitude: %f, Latitude: %f) with key: %s",
        req.Distance, req.Longitude, req.Latitude, key)

	if req.Distance == 0 {
		req.Distance = 3
	}

    geoRadiusQuery := redis.GeoRadiusQuery{
        Radius:   req.Distance,
        WithDist: true,
        Sort:     "ASC",
        Count:    int(end),
    }

    // Query Redis for nearby locations
    locations, err := RedisClient.GeoRadius(ctx, key, req.Longitude, req.Latitude, &geoRadiusQuery).Result()

    if err != nil {
        hlog.Errorf("Error querying Redis: %v", err)
        return nil, err
    }

    // Log the result of the Redis query
    hlog.Infof("Successfully queried Redis. Found %d locations.", len(locations))

    if len(locations) == 0 {
        hlog.Warn("No shops found within the specified radius.")
        return &[]*shop.Shop{}, nil
    }

    // Ensure we're paginating correctly
    if len(locations) <= int(from) {
        hlog.Warnf("No shops found for the requested page (from: %d, end: %d)", from, end)
        return &[]*shop.Shop{}, nil
    }

    // Prepare the result
    shops := make([]*shop.Shop, 0, len(locations))
    for _, loc := range locations[from:end] {
        id, _ := strconv.ParseInt(loc.Name, 10, 64)
        shop := &shop.Shop{
            ID:       id,
            X:        loc.Longitude,
            Y:        loc.Latitude,
            Distance: loc.Dist,
            TypeId:   int64(req.TypeId),
        }
        shops = append(shops, shop)

        // Log each shop added
        hlog.Debugf("Added shop to result: ID=%d, Longitude=%f, Latitude=%f, Distance=%f",
            shop.ID, shop.X, shop.Y, shop.Distance)
    }

    // Return the final result
    hlog.Infof("Returning %d shops.", len(shops))
    return &shops, nil
}

