package hook

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/axetroy/hooker/internal/app/container"
	irisContext "github.com/kataras/iris/v12/context"
	"github.com/pkg/errors"
)

type Repository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
}

type Owner struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarUrl string `json:"avatar_url"`
}

type GithubHookPostData struct {
	Ref        string     `json:"ref"`
	Before     string     `json:"before"`
	After      string     `json:"after"`
	Repository Repository `json:"repository"`
}

type GithubRouterQuery struct {
	Port []string `url:"port"` // 端口映射, 格式为 8080:80, 本机端口:容器端口
	Auth string   `url:"auth"` // 认证方式, basic://username:password 或者 token://xxxxxx
}

// 解析端口
func (q GithubRouterQuery) ParsePort() (ports []container.ExposePort, err error) {
	var (
		machinePort   uint64
		containerPort uint64
	)

	for _, p := range q.Port {
		arr := strings.Split(p, ":")

		machinePort, err = strconv.ParseUint(arr[0], 0, 0)

		if err != nil {
			err = errors.WithStack(err)
			return
		}

		containerPort, err = strconv.ParseUint(arr[1], 0, 0)

		if err != nil {
			err = errors.WithStack(err)
			return
		}

		ports = append(ports, container.ExposePort{
			MachinePort:   machinePort,
			ContainerPort: containerPort,
		})
	}

	return
}

// 解析认证方式，用于克隆项目，公开项目不需要设置，私有项目需要设置
func (q GithubRouterQuery) ParseAuth() (username string, password string, secretKey string, err error) {
	if q.Auth == "" {
		return
	}

	b, err := base64.URLEncoding.DecodeString(q.Auth)

	if err != nil {
		err = errors.WithStack(err)
		return
	}

	reg, err := regexp.CompilePOSIX(`^(basic|token)://(.{3,})$`)

	if err != nil {
		err = errors.WithStack(err)
		return
	}

	matchers := reg.FindAllStringSubmatch(string(b), 1)

	if len(matchers) == 0 {
		err = errors.New("invalid format of auth")
		return
	}

	matcher := matchers[0]

	if len(matcher) != 3 {
		err = errors.New("invalid format of auth")
		return
	}

	schema := matcher[1]
	value := matcher[2]

	switch schema {
	// basic://username:password
	case "basic":
		arr := strings.Split(value, ":")
		username = arr[0]
		password = strings.Join(arr[1:], ":")
	// token://the_token_str
	case "token":
		secretKey = value
	}

	return
}

func GithubRouter(ctx irisContext.Context) {
	var (
		err         error
		data        GithubHookPostData
		query       GithubRouterQuery
		ports       []container.ExposePort
		username    string
		password    string
		accessToken string
		runtime     *container.Runtime
	)

	defer func() {
		if err != nil {
			ctx.StatusCode(http.StatusBadRequest)
			msg := fmt.Sprintf("%+v", err)
			_, _ = ctx.WriteString(msg)
		} else {
			_, _ = ctx.WriteString("Success!")
		}
	}()

	// event:
	// ping
	// push
	event := ctx.GetHeader("X-GitHub-Event")

	if err = ctx.ReadQuery(&query); err != nil {
		err = errors.WithStack(err)
		return
	}

	ports, err = query.ParsePort()

	if err != nil {
		err = errors.WithStack(err)
		return
	}

	username, password, accessToken, err = query.ParseAuth()

	if err != nil {
		err = errors.WithStack(err)
		return
	}

	switch event {
	case "push":
		if err = ctx.ReadJSON(&data); err != nil {
			err = errors.WithStack(err)
			return
		}

		name := fmt.Sprintf("github.com/%s", data.Repository.FullName)

		asyncErr := make(chan error)

		c, cancel := context.WithTimeout(context.Background(), time.Minute*30)

		defer cancel()

		runtime, err = container.NewRuntime(name, data.After, ports)

		if err != nil {
			err = errors.WithStack(err)
			return
		}

		err = runtime.Run(c, username, password, accessToken, asyncErr)

		if err != nil {
			return
		}

		go func() {
			e := <-asyncErr

			log.Printf("%+v\n", e)
		}()

		select {
		case <-time.After(1 * time.Minute * 30):
			err = errors.New("Timeout")
		case <-c.Done():
			err = errors.WithStack(c.Err())
		}
	default:
		err = errors.Errorf("Invalid event '%s'", event)
	}

}
