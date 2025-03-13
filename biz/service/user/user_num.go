package user

import (
	"context"
	"errors"

	"xzdp/biz/dal/mysql"
	user "xzdp/biz/model/user"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

type UserNumService struct {
	RequestContext *app.RequestContext
	Context        context.Context
}

func NewUserNumService(Context context.Context, RequestContext *app.RequestContext) *UserNumService {
	return &UserNumService{RequestContext: RequestContext, Context: Context}
}

func (h *UserNumService) Run(userID int64) (resp *user.UserDTO, err error) {
	// 1. 获取请求的用户ID
    hlog.CtxInfof(h.Context, "Received request to fetch user with ID: %d", userID) // 记录日志

    // 2. 从数据库查询用户信息
    var user user.UserDTO
    result := mysql.DB.Where("id = ?", userID).First(&user)

    // 3. 检查查询是否有错误
    if result.Error != nil {
        if result.Error == gorm.ErrRecordNotFound {
            hlog.CtxErrorf(h.Context, "User with ID %d not found", userID) // 用户未找到的日志
            return nil, errors.New("用户不存在")
        }
        // 记录其他错误
        hlog.CtxErrorf(h.Context, "Error fetching user with ID %d: %v", userID, result.Error)
        return nil, result.Error
    }

    // 4. 返回查询到的用户信息
    hlog.CtxInfof(h.Context, "Successfully fetched user: %+v", user) // 查询成功的日志
    return &user, nil
}
