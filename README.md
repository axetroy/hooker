[![Build Status](https://github.com/axetroy/hooker/workflows/ci/badge.svg)](https://github.com/axetroy/hooker/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/axetroy/hooker)](https://goreportcard.com/report/github.com/axetroy/hooker)
![Latest Version](https://img.shields.io/github/v/release/axetroy/hooker.svg)
![License](https://img.shields.io/github/license/axetroy/hooker.svg)
![Repo Size](https://img.shields.io/github/repo-size/axetroy/hooker.svg)

## Hooker

> 当前仍然出于开发阶段

Hook 是一个基于 Docker 的自动化部署工具，根据定义好的 Dockerfile 进行构建并且部署。

这是一个简易的自动化部署工具，功能肯定无法与 `JenKins` 相比。

但是有时候我们就希望有一个简单的自动化部署，而不用配置 `JenKins` 一大堆啰嗦的东西。

改工具仅适用于部署到测试服务器上

特性:

- [x] 部署镜像到本地
- [ ] 部署镜像到远程服务器
- [ ] 支持 `docker-compose.yml`

不会支持的特性:

1. ~~构建日志~~
2. ~~添加数据库/消息队列等第三方服务的支持~~

### 使用

把 URL 添加到仓库的 Web Hook 中

```
https://你的域名/v1/hook/github.com
```

### 当前工作原理

1. 仓库 push 触发 web hook
2. 程序接收到构建通知
3. 克隆项目
    if 如果项目已存在
        删除项目目录
        尝试删除对应的镜像/容器

    根据 hash 克隆项目
4. 根据 Dockerfile 构建一个新的镜像
5. 停止已经在运行的旧容器
6. 启动新镜像
    6.1 删除旧容器
    6.2 删除旧镜像
7. 接口返回 success

### 重构新流程

在后续的版本中，应该重构成这个流程

主程序:
1. 仓库 push 触发 webhook
2. 程序接收到构建通知
3. 把构建情况加入到消息队列

消息队列:
1. 首到新的构建消息
2. 克隆项目
3. 根据 Dockerfile 构建一个新的镜像
4. 停止已经在运行的旧镜像
5. 启动新镜像
6. 完成

### Q & A

1. 如何构建私有项目?

目前支持构建公开的项目，构建私有项目则需要认证

> 这种把认证 token 直接放在 URL 上是具有安全隐患的

```
https://你的域名/v1/hook/github.com?auth=xxxx
```

其中 auth 是 base64 转码之后的字符串， 有两种格式

```
basic://username:password
token://you_access_token
```

2. 如果暴露容器的端口

通过参数`?port=1234:1234`

```
https://你的域名/v1/hook/github.com?auth=xxxx&port=1234:1234
```

可以同时暴露多个端口`?port=1234:1234&port=2345:2345`

here is an [example](https://github.com/axetroy/hooker-example)