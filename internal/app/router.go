// Copyright 2019-2020 Axetroy. All rights reserved. MIT license.
package app

import (
	"html/template"

	"github.com/axetroy/hooker/internal/app/hook"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
)

var Router *iris.Application

func init() {
	app := iris.New()

	// 接口
	{
		v1 := app.Party("v1").AllowMethods(iris.MethodOptions)
		//v1.Use(logger.New())
		{
			authRouter := v1.Party("/auth")
			authRouter.Post("/register", func(c context.Context) {}) // 注册帐号
			authRouter.Post("/login", func(c context.Context) {})    // 登录帐号
		}

		{
			hookRouter := v1.Party("/hook")
			hookRouter.Post("/{project}", func(context context.Context) {})                 // 触发项目的钩子
			hookRouter.Post("/github.com/{owner}/{repo}", hook.GithubRouter)                // 单独部署 Github
			hookRouter.Post("/gitlab.com/{owner}/{repo}", func(context context.Context) {}) // 单独部署 Gitlab
			hookRouter.Post("/gogs.com/{owner}/{repo}", func(context context.Context) {})   // 单独部署 gogs
			hookRouter.Post("/gitea.com/{owner}/{repo}", func(context context.Context) {})  // 单独部署 gitea
			hookRouter.Post("/gitee.com/{owner}/{repo}", func(context context.Context) {})  // 单独部署 Gitee
		}

		{
			projectRouter := v1.Party("/project")
			projectRouter.Post("", func(c context.Context) {})        // 创建项目
			projectRouter.Put("/{id}", func(c context.Context) {})    // 更新项目
			projectRouter.Get("/{id}", func(c context.Context) {})    // 获取项目
			projectRouter.Get("/", func(c context.Context) {})        // 获取列表
			projectRouter.Delete("/{id}", func(c context.Context) {}) // 删除项目

			{
				logRouter := projectRouter.Party("/{project}/log")
				logRouter.Get("", func(c context.Context) {})      // 项目部署日志列表
				logRouter.Get("/{id}", func(c context.Context) {}) // 项目部署日志详情
			}
		}
	}

	// 视图
	{
		app.Get("/login", func(c context.Context) {
			t, _ := template.ParseFiles("./internal/app/views/login.html")
			_ = t.ExecuteTemplate(c.ResponseWriter(), "login", map[string]interface{}{
				"Title": "view",
			})
		})

		app.Get("/", func(c context.Context) {
			t, _ := template.ParseFiles("./internal/app/views/layout.html", "./internal/app/views/index.html")
			_ = t.ExecuteTemplate(c.ResponseWriter(), "layout", "Hello world")
		})
	}

	_ = app.Build()

	Router = app
}
