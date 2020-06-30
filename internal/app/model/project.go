package model

type Project struct {
	Id         string `json:"id"`         // 项目 ID
	Name       string `json:"name"`       // 项目名称
	Desc       string `json:"desc"`       // 项目描述
	Dockerfile string `json:"dockerfile"` // 指定的 Dockerfile 文件内容
	Hosts      []Host `json:"hosts"`      // 部署到对应的服务器
}

type Host struct {
	Id         string `json:"id"`          // 服务器 ID
	Host       string `json:"host"`        // 服务器地址
	Port       uint32 `json:"port"`        // 端口
	Username   string `json:"username"`    // 用户名
	Password   string `json:"password"`    // 密码
	PrivateKey string `json:"private_key"` // 私钥
}
