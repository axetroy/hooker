package container

import (
	"context"
	"fmt"
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
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
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
}

func NewRuntime(repo string, hash string, ports []ExposePort) (*Runtime, error) {
	cli, err := client.NewEnvClient()

	if err != nil {
		return nil, errors.WithStack(err)
	}

	r := Runtime{
		repo:   repo,
		hash:   hash,
		ports:  ports,
		client: cli,
	}

	return &r, nil
}

func (r *Runtime) beforeRun(ctx context.Context) error {
	// stop all container run before
	containers, err := r.client.ContainerList(ctx, types.ContainerListOptions{})

	if err != nil {
		return errors.WithStack(err)
	}

	for _, a := range containers {
		e := func(c types.Container) error {
			if strings.HasPrefix(c.Image, r.repo+":") {
				// kill container
				log.Printf("Stop container '%s'\n", c.ID)

				{
					stopCommand := exec.Command("docker", "stop", c.ID)

					stopCommand.Stdout = os.Stdout
					stopCommand.Stderr = os.Stderr

					if err := stopCommand.Start(); err != nil {
						return errors.WithStack(err)
					}
				}

				//timeout := 2 * time.Minute
				//
				//if err := r.client.ContainerStop(ctx, c.ID, &timeout); err != nil {
				//	return errors.WithStack(err)
				//}

				log.Printf("Remove container '%s'\n", c.ID)

				{
					rmCommand := exec.Command("docker", "rm", "-f", c.ID)

					rmCommand.Stdout = os.Stdout
					rmCommand.Stderr = os.Stderr

					if err := rmCommand.Start(); err != nil {
						return errors.WithStack(err)
					}
				}

				//if err := r.client.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{
				//	RemoveVolumes: true,
				//	RemoveLinks:   true,
				//	Force:         true,
				//}); err != nil {
				//	return errors.WithStack(err)
				//}
				log.Printf("Remove container '%s' success\n", c.ID)
			}

			return nil
		}(a)

		if e != nil {
			return errors.WithStack(e)
		}
	}

	return nil
}

func (r *Runtime) clone(c context.Context, username string, password string, accessToken string, hash string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)

	defer cancel()

	// clone project
	fs := osfs.New(path.Join("repos", r.repo, r.hash))

	options := git.CloneOptions{
		URL:      getGitURL(r.repo, username, password),
		Progress: os.Stdout,
		//SingleBranch:      true,
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

func (r *Runtime) Run(ctx context.Context, username string, password string, accessToken string, ch chan error) error {
	var (
		rootPath string
	)
	if p, err := r.clone(ctx, username, password, accessToken, r.hash); err != nil {
		return errors.WithStack(err)
	} else {
		rootPath = p
	}

	reader, err := archive.TarWithOptions(rootPath, &archive.TarOptions{})

	imageName := fmt.Sprintf("%s:%s", r.repo, r.hash)

	options := types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Tags:           []string{imageName},
	}

	log.Println("Building image...")

	buildResponse, err := r.client.ImageBuild(context.Background(), reader, options)

	if err != nil {
		return errors.WithStack(err)
	}

	// stop all container run before
	if err := r.beforeRun(ctx); err != nil {
		return errors.WithStack(err)
	}

	defer func() {
		_ = buildResponse.Body.Close()

		if list, err := r.client.ImageList(ctx, types.ImageListOptions{}); err != nil {
			//return err
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
					// return err
				}
			}
		}
	}()

	// Copy out response of stream
	_, err = io.Copy(os.Stdout, buildResponse.Body)

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

	defer func() {
		_ = r.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{
			RemoveVolumes: true,
			RemoveLinks:   true,
			Force:         true,
		})
	}()

	if err := r.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return errors.WithStack(err)
	}

	log.Printf("Start container '%s'\n", imageName)

	go func() {
		var err error
		_, err = r.client.ContainerWait(ctx, resp.ID)

		if err != nil {
			err = errors.WithStack(err)
		}

		defer func() {
			ch <- err
		}()
	}()

	return nil
}
