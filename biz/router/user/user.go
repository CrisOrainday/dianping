// Code generated by hertz generator. DO NOT EDIT.

package user

import (
	"github.com/cloudwego/hertz/pkg/app/server"
	user "xzdp/biz/handler/user"
)

/*
 This file will register all the routes of the services in the master idl.
 And it will update automatically when you use the "update" command for the idl.
 So don't modify the contents of the file, or your code will be deleted when it is updated.
*/

// Register register routes based on the IDL 'api.${HTTP Method}' annotation.
func Register(r *server.Hertz) {

	root := r.Group("/", rootMw()...)
	{
		_user := root.Group("/user", _userMw()...)
		_user.POST("/code", append(_sendcodeMw(), user.SendCode)...)
		_user.GET("/:id", append(_usernumMw(), user.UserNum)...)
		_user.POST("/login", append(_userloginMw(), user.UserLogin)...)
		_user.GET("/me", append(_usermeMw(), user.UserMe)...)
		_user.POST("/sign", append(_usersignMw(), user.UserSign)...)
		_sign := _user.Group("/sign", _signMw()...)
		_sign.GET("/count", append(_usersigncountMw(), user.UserSignCount)...)
		{
			_info := _user.Group("/info", _infoMw()...)
			_info.GET("/:id", append(_userinfoMw(), user.UserInfo)...)
		}
	}
}
