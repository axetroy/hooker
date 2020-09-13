package container

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/pkg/errors"
)

func getGitURL(repo, username, password string) string {
	switch true {
	case strings.HasPrefix(repo, "github.com"):
		if username != "" {
			return fmt.Sprintf("https://%s:%s@%s.git", url.QueryEscape(username), url.QueryEscape(password), repo)
		} else {
			return fmt.Sprintf("https://%s.git", repo)
		}
	case strings.HasPrefix(repo, "gitlab.com"):
		// TODO: finish gitlab
		break
	}

	return ""
}

type ExposePort struct {
	MachinePort   uint64 // 机器的端口
	ContainerPort uint64 // 容器的端口
}

type Runtime struct {
	repo   string
	hash   string
	ports  []ExposePort
	client *client.Client
	writer io.Writer
}

func NewRuntime(repo string, hash string, ports []ExposePort, writer io.Writer) (*Runtime, error) {
	cli, err := client.NewEnvClient()

	if err != nil {
		return nil, errors.WithStack(err)
	}

	r := Runtime{
		repo:   repo,
		hash:   hash,
		ports:  ports,
		client: cli,
		writer: writer,
	}

	return &r, nil
}

// stop all container run before
func (r *Runtime) beforeRun(ctx context.Context) error {
	containers, err := r.client.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})

	if err != nil {
		return errors.WithStack(err)
	}

	for _, a := range containers {
		e := func(c types.Container) error {
			if strings.HasPrefix(c.Image, r.repo+":") {
				// kill container
				log.Printf("Stoping container '%s'\n", c.ID)

				//{
				//	stopCommand := exec.Command("docker", "stop", c.ID)
				//
				//	//stopCommand.Stdout = os.Stdout
				//	//stopCommand.Stderr = os.Stderr
				//
				//	if err := stopCommand.Run(); err != nil {
				//		return errors.WithStack(err)
				//	}
				//}

				timeout := 10 * time.Second

				if err := r.client.ContainerStop(ctx, c.ID, &timeout); err != nil {
					return errors.WithStack(err)
				}
			}

			return nil
		}(a)

		if e != nil {
			return errors.WithStack(e)
		}
	}

	return nil
}

func (r *Runtime) afterRun(ctx context.Context, imageName string) error {
	if list, err := r.client.ImageList(ctx, types.ImageListOptions{}); err != nil {
		return err
	} else {
		var imageId string

		for _, img := range list {
			for _, tag := range img.RepoTags {
				if tag == imageName {
					imageId = img.ID
				}
			}
		}

		if len(imageId) > 0 {
			if _, err := r.client.ImageRemove(ctx, imageId, types.ImageRemoveOptions{
				Force:         true,
				PruneChildren: true,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Runtime) clone(ctx context.Context, username string, password string, accessToken string, hash string) (string, error) {
	var (
		err error
	)

	// clone project
	fs := osfs.New(path.Join("repos", r.repo, r.hash))

	if _, e := os.Stat(fs.Root()); e == nil {
		// if folder exist. then remove it first
		if err = os.RemoveAll(fs.Root()); err != nil {
			return "", errors.WithStack(err)
		}
	} else if !os.IsNotExist(e) {
		return "", errors.WithStack(e)
	}

	options := git.CloneOptions{
		URL:               getGitURL(r.repo, username, password),
		Progress:          os.Stdout,
		SingleBranch:      true,
		Depth:             1,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	}

	if password != "" {
		options.Auth = &http.BasicAuth{
			Username: username,
			Password: password,
		}
		options.URL = getGitURL(r.repo, "", "")
	} else if accessToken != "" {
		options.Auth = &http.BasicAuth{
			Username: "access",
			Password: accessToken,
		}
		options.URL = getGitURL(r.repo, "", "")
	}

	gitDir := path.Join("./", fs.Root(), ".git")

	repo, err := git.CloneContext(ctx, filesystem.NewStorage(osfs.New(gitDir), cache.NewObjectLRU(cache.MiByte*50)), fs, &options)

	defer func() {
		if err != nil {
			_ = os.RemoveAll(fs.Root())
		}
	}()

	if err != nil {
		return "", errors.WithStack(err)
	}

	tree, err := repo.Worktree()

	if err != nil {
		return "", errors.WithStack(err)
	}

	if err := tree.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(hash),
		Force: true,
		Keep:  true,
	}); err != nil {
		return "", errors.WithStack(err)
	}

	return fs.Root(), nil
}

func (r *Runtime) buildImage(ctx context.Context, rootPath string, imageName string) (io.ReadCloser, error) {
	reader, err := archive.TarWithOptions(rootPath, &archive.TarOptions{})

	options := types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Tags:           []string{imageName},
	}

	log.Println("Building image...")

	buildResponse, err := r.client.ImageBuild(ctx, reader, options)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return buildResponse.Body, nil
}

func (r *Runtime) Run(ctx context.Context, username string, password string, accessToken string, ch chan error) error {
	var (
		rootPath string
	)
	if p, err := r.clone(ctx, username, password, accessToken, r.hash); err != nil {
		return errors.WithStack(err)
	} else {
		rootPath = p
	}

	imageName := fmt.Sprintf("%s:%s", r.repo, r.hash)

	output, err := r.buildImage(ctx, rootPath, imageName)

	if err != nil {
		return errors.WithStack(err)
	}

	// copy out response of stream
	_, err = io.Copy(r.writer, output)

	defer func() {
		_ = output.Close()
	}()

	// stop all container run before
	if err := r.beforeRun(ctx); err != nil {
		return errors.WithStack(err)
	}

	// remove all images create before
	defer func() {
		if err != nil {
			if er := r.afterRun(ctx, imageName); err != nil {
				err = er
			}
		}
	}()

	if err != nil {
		return errors.WithStack(err)
	}

	portMap := nat.PortMap{}
	exposedPorts := nat.PortSet{}

	for _, p := range r.ports {
		portMap[nat.Port(fmt.Sprintf("%d/tcp", p.ContainerPort))] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", p.MachinePort)},
		}
		exposedPorts[nat.Port(fmt.Sprintf("%d/tcp", p.ContainerPort))] = struct{}{}
	}

	hostConfig := &container.HostConfig{
		PortBindings: portMap,
		AutoRemove:   true,
	}

	resp, err := r.client.ContainerCreate(ctx, &container.Config{
		Image:        imageName,
		ExposedPorts: exposedPorts,
	}, hostConfig, nil, "")

	if err != nil {
		return errors.WithStack(err)
	}

	if err := r.client.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		return errors.WithStack(err)
	}

	log.Printf("Start container '%s'\n", imageName)

	go func() {
		var err error
		// wait until container exit
		_, err = r.client.ContainerWait(context.Background(), resp.ID)

		if err != nil {
			err = errors.WithStack(err)
		}

		defer func() {
			ch <- err
		}()
	}()

	return nil
}
