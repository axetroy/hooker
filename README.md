[![Build Status](https://github.com/axetroy/hooker/workflows/ci/badge.svg)](https://github.com/axetroy/hooker/actions)
[![Docker Build Status](https://img.shields.io/docker/cloud/build/axetroy/hooker)](https://hub.docker.com/r/axetroy/hooker/builds)
[![Docker Pulls](https://img.shields.io/docker/pulls/axetroy/hooker)](https://hub.docker.com/r/axetroy/hooker/builds)
[![Go Report Card](https://goreportcard.com/badge/github.com/axetroy/hooker)](https://goreportcard.com/report/github.com/axetroy/hooker)
![Latest Version](https://img.shields.io/github/v/release/axetroy/hooker.svg)
![License](https://img.shields.io/github/license/axetroy/hooker.svg)
![Repo Size](https://img.shields.io/github/repo-size/axetroy/hooker.svg)

![screen](./screen.png)

## Hooker

> 当前仍然出于开发阶段

Hook 是一个基于 Docker 的自动化部署工具，根据定义好的 Dockerfile 进行构建并且部署。

这是一个简易的自动化部署工具，功能肯定无法与 `JenKins` 相比。

但是有时候我们就希望有一个简单的自动化部署，而不用配置 `JenKins` 一大堆啰嗦的东西。

而且该工具基于 Docker 进行打包构建，程序运行的环境是在容器中，是隔离的。

该工具仅适用于部署到测试服务器上

特性:

- [x] 部署镜像到本地
- [ ] 部署镜像到远程服务器
- [ ] 支持 `docker-compose.yml`

不会支持的特性:

1. ~~构建日志~~
2. ~~添加数据库/消息队列等第三方服务的支持~~

### 安装

如果你是 Linux/macOS 系统，从以下命令中安装

```bash
# 安装最新版
curl -fsSL https://raw.githubusercontent.com/axetroy/hooker/master/install.sh | bash
# 安装指定版本
curl -fsSL https://raw.githubusercontent.com/axetroy/hooker/master/install.sh | bash -s v1.0.0
```

如果是 Window 系统，从 [Github release page](https://github.com/axetroy/hooker/releases) 中下载

### 使用

把 URL 添加到仓库的 Web Hook 中

```
https://你的域名/v1/hook/github.com
```

### 当前工作原理

1. 仓库 push 触发 web hook
2. 程序接收到构建通知
3. 克隆项目

   ```js
   if (/* 如果项目已存在 */) {
      // 1. 删除项目目录
      // 2. 尝试删除对应的镜像/容器
   }

   // 3. 根据 hash 克隆项目
   ```

4. 根据 Dockerfile 构建一个新的镜像
5. 停止已经在运行的旧容器
6. 启动新镜像

   6.1 删除旧容器

   6.2 删除旧镜像

7. 接口返回 success

### Q & A

1. 如何构建私有项目？

目前支持构建公开的项目，构建私有项目则需要认证

> 这种把认证 token 直接放在 URL 上是具有安全隐患的

```
https://你的域名/v1/hook/github.com?auth=xxxx
```

其中 auth 是 base64 转码之后的字符串， 有两种格式

```
basic://username:password
token://your_access_token
```

2. 如何暴露容器的端口？

通过参数`?port=1234:1234`

```
https://你的域名/v1/hook/github.com?auth=xxxx&port=1234:1234
```

可以同时暴露多个端口`?port=1234:1234&port=2345:2345`

这里有一个例子 https://github.com/axetroy/hooker-example

### License

The MIT License
