package blog

import (
	"context"

	"xzdp/biz/dal/mysql"
	blog "xzdp/biz/model/blog"
	"xzdp/biz/model/user"

	"github.com/cloudwego/hertz/pkg/app"
)

type GetTheUserBlogService struct {
	RequestContext *app.RequestContext
	Context        context.Context
}

func NewGetTheUserBlogService(Context context.Context, RequestContext *app.RequestContext) *GetTheUserBlogService {
	return &GetTheUserBlogService{RequestContext: RequestContext, Context: Context}
}

func (h *GetTheUserBlogService) Run(req *blog.BlogReq) (resp *[]*blog.Blog, err error) {
	//defer func() {
	// hlog.CtxInfof(h.Context, "req = %+v", req)
	// hlog.CtxInfof(h.Context, "resp = %+v", resp)
	//}()
	// todo edit your code
	u, err := mysql.GetById(h.Context, req.ID)
	if err != nil {
		return nil, err
	}
	d := &user.UserDTO{
		ID:       u.ID,
		NickName: u.NickName,
		Icon:     u.Icon,
	}
	blogList, err := mysql.QueryBlogByUserID(h.Context, int(req.Current), d)
	if err != nil {
		return nil, err
	}
	return &blogList, nil
}
