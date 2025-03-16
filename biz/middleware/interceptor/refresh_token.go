package interceptor

import (
	"context"
	"strings"
	"time"
	"xzdp/biz/dal/redis"
	model "xzdp/biz/model/user"
	"xzdp/biz/pkg/constants"
	"xzdp/biz/utils"
	"xzdp/conf"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func CheckToken(ctx context.Context, c *app.RequestContext) {
	hlog.CtxInfof(ctx, "check token interceptor:%+v", conf.GetEnv())
	//if conf.GetEnv() != "online" {
	//	userdto := model.UserDTO{
	//		ID:       2,
	//		NickName: "法外狂徒张三",
	//		Icon:     "https://img2.baidu.com/it/u=194756667,2850459164&fm=253&fmt=auto&app=138&f=JPEG?w=500&h=500",
	//	}
	//	ctx = utils.SaveUser(ctx, &userdto)
	//	c.Next(ctx)
	//	return
	//}
	token := c.GetHeader("authorization")
	tokenStr := string(token)
		
	// 如果 token 是以 'Bearer ' 开头，提取实际的 token
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	token = []byte(tokenStr)
	if token == nil {
		c.Next(ctx)
	}
	hlog.CtxInfof(ctx, "token = %s", token)
	if len(token) == 0 {
		c.Next(ctx)
	}
	var userdto model.UserDTO
	if err := redis.SlaveRedisClient.HGetAll(ctx, constants.LOGIN_USER_KEY+string(token)).Scan(&userdto); err != nil {
		c.Next(ctx)
	}
	if userdto == (model.UserDTO{}) {
		c.Next(ctx)
	}
	redis.MasterRedisClient.Expire(ctx, constants.LOGIN_USER_KEY+string(token), time.Minute*30)
	ctx = utils.SaveUser(ctx, &userdto)
	c.Next(ctx)
	if utils.GetUser(ctx) == nil {
		hlog.CtxErrorf(ctx, "check token interceptor error")
	}
	hlog.CtxDebugf(ctx, "user = %+v", utils.GetUser(ctx))
}

func LoginInterceptor(ctx context.Context, c *app.RequestContext) {
	hlog.CtxInfof(ctx, "login interceptor")
	if conf.GetEnv() == "dev" {
		c.Next(ctx)
		return
	}
	if utils.GetUser(ctx) == nil {
		c.SetStatusCode(401)
		c.Abort()
	}
	c.Next(ctx)
}
