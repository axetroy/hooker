package hook

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
	Port []string `json:"port"` // 端口映射
}

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

func GithubRouter(ctx irisContext.Context) {
	var (
		err   error
		data  GithubHookPostData
		query GithubRouterQuery
	)

	defer func() {
		if err != nil {
			ctx.StatusCode(http.StatusBadRequest)
			_, _ = ctx.WriteString(fmt.Sprintf("%+v", err))
		} else {
			_, _ = ctx.WriteString("Success!")
		}
	}()

	// event:
	// ping
	// push
	//event := ctx.GetHeader("X-GitHub-Event")

	owner := ctx.Params().Get("owner")
	repo := ctx.Params().Get("repo")

	name := fmt.Sprintf("github.com/%s/%s", owner, repo)

	if err = ctx.ReadQuery(&query); err != nil {
		err = errors.WithStack(err)
		return
	}

	fmt.Printf("%+v\n", query)

	if err = ctx.ReadJSON(&data); err != nil {
		err = errors.WithStack(err)
		return
	}

	asyncErr := make(chan error)

	c, cancel := context.WithTimeout(context.Background(), time.Minute*30)

	defer cancel()

	ports, err := query.ParsePort()

	if err != nil {
		err = errors.WithStack(err)
		return
	}

	runtime, err := container.NewRuntime(name, data.After, ports)

	if err != nil {
		err = errors.WithStack(err)
		return
	}

	err = runtime.Run(c, asyncErr)

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
}
