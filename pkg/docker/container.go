package docker

import (
	"context"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	dockernetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/fyve-labs/fyve-cli/pkg/docker/images"
	"github.com/pkg/errors"
	"io"
	"log/slog"
)

type ContainerService struct {
	client         *client.Client
	registryClient *images.RegistryClient
}

func NewContainerService(endpoint string) (*ContainerService, error) {
	opts := make([]client.Opt, 0)
	if endpoint != "" {
		opts = append(opts, client.WithHost(endpoint))
	}

	opts = append(opts, client.FromEnv, client.WithAPIVersionNegotiation())
	dockerClient, err := client.NewClientWithOpts(
		opts...,
	)

	if err != nil {
		return nil, errors.Wrap(err, "create client error")
	}

	registry, err := images.NewRegistryClient()
	if err != nil {
		return nil, errors.Wrap(err, "create registry client error")
	}

	return &ContainerService{
		client:         dockerClient,
		registryClient: registry,
	}, nil
}

func (c *ContainerService) ReCreate(ctx context.Context, containerNameOrId string, forcePullImage bool, imageTag string) (*dockercontainer.InspectResponse, error) {
	container, err := c.client.ContainerInspect(ctx, containerNameOrId)
	if err != nil {
		return nil, errors.Wrap(err, "fetch container information error")
	}

	img, err := images.ParseImage(images.ParseImageOptions{
		Name: container.Config.Image,
	})
	if err != nil {
		return nil, errors.Wrap(err, "parse image error")
	}

	if imageTag != "" {
		if err := img.WithTag(imageTag); err != nil {
			return nil, errors.Wrapf(err, "set image tag error %s", imageTag)
		}

		container.Config.Image = img.FullName()
	}

	containerId := container.ID

	// 1. pull image if you need force pull
	if forcePullImage {
		if err := c.Pull(ctx, img); err != nil {
			return nil, errors.Wrapf(err, "pull image error %s", img.FullName())
		}
	}

	// 2. stop the current container
	slog.Debug("starting to stop the container")
	if err := c.client.ContainerStop(ctx, containerId, dockercontainer.StopOptions{}); err != nil {
		return nil, errors.Wrap(err, "stop container error")
	}

	// 3. rename the current container
	slog.Debug("starting to rename the container")
	if err := c.client.ContainerRename(ctx, containerId, container.Name+"-old"); err != nil {
		return nil, errors.Wrap(err, "rename container error")
	}

	initialNetwork := dockernetwork.NetworkingConfig{
		EndpointsConfig: make(map[string]*dockernetwork.EndpointSettings),
	}

	// 4. disconnect all networks from the current container
	for name, network := range container.NetworkSettings.Networks {
		// This allows new container to use the same IP address if specified
		if err := c.client.NetworkDisconnect(ctx, network.NetworkID, containerId, true); err != nil {
			return nil, errors.Wrap(err, "disconnect network from old container error")
		}

		// 5. get the first network attached to the current container
		if len(initialNetwork.EndpointsConfig) == 0 {
			// Retrieve the first network that is linked to the present container, which
			// will be utilized when creating the container.
			initialNetwork.EndpointsConfig[name] = network
		}
	}

	// 6. create a new container
	create, err := c.client.ContainerCreate(ctx, container.Config, container.HostConfig, &initialNetwork, nil, container.Name)
	if err != nil {
		return nil, errors.Wrap(err, "create container error")
	}

	newContainerId := create.ID
	// 7. connect to networks
	// docker can connect to only one network at creation, so we need to connect to networks after creation
	// see https://github.com/moby/moby/issues/17750
	slog.Debug("connecting networks to container")
	networks := container.NetworkSettings.Networks
	for key, network := range networks {
		if _, ok := initialNetwork.EndpointsConfig[key]; ok {
			// skip the network that is used during container creation
			continue
		}

		if err := c.client.NetworkConnect(ctx, network.NetworkID, newContainerId, network); err != nil {
			return nil, errors.Wrap(err, "connect container network error")
		}
	}

	// 8. start the new container
	slog.Debug("starting the new container")
	if err := c.client.ContainerStart(ctx, newContainerId, dockercontainer.StartOptions{}); err != nil {
		return nil, errors.Wrap(err, "start container error")
	}

	// 9. delete the old container
	slog.Debug("starting to remove the old container")
	_ = c.client.ContainerRemove(ctx, containerId, dockercontainer.RemoveOptions{})

	newContainer, _, err := c.client.ContainerInspectWithRaw(ctx, newContainerId, true)
	if err != nil {
		return nil, errors.Wrap(err, "fetch new container information error")
	}

	return &newContainer, nil
}

func (c *ContainerService) Pull(ctx context.Context, img images.Image) error {
	slog.Debug("Pulling image...", slog.String("image", img.FullName()))
	registryAuth, err := c.registryClient.EncodedRegistryAuth(ctx, img)
	if err != nil {
		return err
	}

	out, err := c.client.ImagePull(ctx, img.FullName(), image.PullOptions{
		RegistryAuth: registryAuth,
	})

	if err != nil {
		return err
	}

	defer func(out io.ReadCloser) {
		_ = out.Close()
	}(out)

	_, err = io.ReadAll(out)

	return err
}
