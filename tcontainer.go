package tcontainer

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	macOSLocalhost = "127.0.0.1"
	macOSName      = "darwin"
	linuxLocalhost = "localhost"
	linuxOSName    = "linux"
)

var (
	ErrRetryTimeout = errors.New("retry timeout")
)

// Endpoint that you can use to connect to the container.
//
// Note: macOS users may encounter issues accessing the container through ApiEndpoint
// from inside the container. This is because macOS users cannot use the container's IP directly,
// potentially leading to connectivity problems.
type ApiEndpoint struct {
	IP   string // localhost/dockerGateway or container IP
	Port int    // publicPort or private port
}

// RetryOperation is an exponential backoff retry operation. You can use it to wait for e.g. mysql to boot up.
//
// `apiEndpoints` is map that provides you ApiEndpoint by each privatePort (port inside the container).
type RetryOperation func(container *dockertest.Resource, apiEndpoints map[int]ApiEndpoint) (err error)

// New - creates a new test container.
func New(
	repository string,
	customOpts ...TestContainerOption,
) (
	dockerPool *dockertest.Pool,
	container *dockertest.Resource,
	err error,
) {

	dockerPool, err = dockertest.NewPool("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dockertest.NewPool: %w", err)
	}

	container, err = NewWithPool(dockerPool, repository, customOpts...)

	return dockerPool, container, err
}

// NewWithPool - creates a new test container using passed dockertest.Pool.
func NewWithPool(
	dockerPool *dockertest.Pool,
	repository string,
	customOpts ...TestContainerOption,
) (
	container *dockertest.Resource,
	err error,
) {
	options, err := applyTestContainerOptions(repository, customOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to applyTestContainerOptions: %w", err)
	}

	return newWithPool(dockerPool, repository, options)
}

func newWithPool(
	dockerPool *dockertest.Pool,
	repository string,
	options *testContainerOptions,
) (
	container *dockertest.Resource,
	err error,
) {
	container, err = createAndStartContainer(dockerPool, repository, options)
	if err != nil {
		return nil, fmt.Errorf("failed to createAndStartContainer: %w", err)
	}

	if options.containerExpiry != 0 {
		err = container.Expire(uint(options.containerExpiry.Seconds()))
		if err != nil {
			_ = dockerPool.Purge(container)
			return nil, fmt.Errorf("failed to container.Expire: %w", err)
		}
	}

	apiEndpoints, err := getApiEndpoints(container)
	if err != nil {
		_ = dockerPool.Purge(container)
		return nil, fmt.Errorf("failed to getApiEndpoints: %w", err)
	}

	if options.retryOperation != nil {
		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = time.Second * 5
		bo.MaxElapsedTime = options.retryTimeout

		err = backoff.Retry(func() (err error) {
			return options.retryOperation(container, apiEndpoints)
		}, bo)
		if err != nil {
			_ = dockerPool.Purge(container)
			if bo.NextBackOff() == backoff.Stop {
				err = fmt.Errorf("%w: %w", ErrRetryTimeout, err)
			}
			return nil, fmt.Errorf("failed to backoff.Retry: %w", err)
		}
	}

	return container, nil
}

func createAndStartContainer(dockerPool *dockertest.Pool, repository string, options *testContainerOptions) (
	container *dockertest.Resource, err error,
) {
	container, err = dockerPool.RunWithOptions(&dockertest.RunOptions{
		Name:         options.containerName,
		Platform:     fmt.Sprintf("linux/%s", runtime.GOARCH),
		Repository:   repository,
		Tag:          options.imageTag,
		Env:          options.env,
		PortBindings: options.portBindings,
		ExposedPorts: options.exposedPorts,
		Cmd:          options.cmd,
	}, func(config *docker.HostConfig) {
		config.AutoRemove = options.autoremoveContainer
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})

	switch {
	case err == nil:
		return container, nil

	case errors.Is(err, docker.ErrContainerAlreadyExists) && options.reuseContainer:
		container, err = reuseContainer(dockerPool, options, false)
		if err != nil {
			err = fmt.Errorf("failed to reuseContainer: %w", err)

			if options.reuseContainerRecreateOnErr {
				container, recreateErr := recreateContainer(dockerPool, repository, options)
				if recreateErr != nil {
					recreateErr = fmt.Errorf("failed to recreateContainer by reuseContainer: %w", err)
					return nil, errors.Join(err, recreateErr)
				}

				return container, nil
			}

			return nil, err
		}

		return container, nil

	case errors.Is(err, docker.ErrContainerAlreadyExists) && options.removeContainerOnExists:
		container, err := recreateContainer(dockerPool, repository, options)
		if err != nil {
			return nil, fmt.Errorf("failed to recreateContainer by removeContainerOnExists: %w", err)
		}

		return container, nil

	default:
		return nil, fmt.Errorf("failed to dockerPool.RunWithOptions: %w", err)
	}
}

func reuseContainer(dockerPool *dockertest.Pool, options *testContainerOptions, isRecursiveCall bool) (container *dockertest.Resource, err error) {
	container, ok := dockerPool.ContainerByName(fmt.Sprintf("^%s$", options.containerName))
	if !ok {
		return nil, fmt.Errorf("failed to dockerPool.ContainerByName `%s`: %w", options.containerName, err)
	}

	err = checkContainerConfig(container, options)
	if err != nil {
		return nil, fmt.Errorf("failed to checkContainerConfig: %w", err)
	}

	retry := func() (container *dockertest.Resource, err error) {
		bo := backoff.NewExponentialBackOff()
		bo.InitialInterval = time.Second
		bo.MaxInterval = time.Second
		bo.MaxElapsedTime = options.reuseContainerTimeout

		err = backoff.Retry(func() (err error) {
			container, err = reuseContainer(dockerPool, options, true)
			if err != nil {
				return fmt.Errorf("failed to reuseContainer: %w", err)
			}

			return nil
		}, bo)
		if err != nil {
			return nil, err
		}

		return container, nil
	}

	switch {
	case container.Container.State.Paused:
		if isRecursiveCall {
			return nil, fmt.Errorf("still paused")
		}

		err = dockerPool.Client.UnpauseContainer(container.Container.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to UnpauseContainer: %w", err)
		}

		container, err = retry()
		if err != nil {
			return nil, fmt.Errorf("failed to retry while State.Restarting: %w", err)
		}

	case container.Container.State.Status == "exited":
		if isRecursiveCall {
			return nil, fmt.Errorf("still exited")
		}

		err = dockerPool.Client.StartContainer(container.Container.ID, container.Container.HostConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to StartContainer on `exited` status: %w", err)
		}

		container, err = retry()
		if err != nil {
			return nil, fmt.Errorf("failed to retry while State.Restarting: %w", err)
		}

	case container.Container.State.Restarting:
		if isRecursiveCall {
			return nil, fmt.Errorf("still restarting")
		}

		container, err = retry()
		if err != nil {
			return nil, fmt.Errorf("failed to retry while State.Restarting: %w", err)
		}

	// TODO: implement and test other states
	// case container.Container.State.OOMKilled:
	// case container.Container.State.RemovalInProgress:
	// case container.Container.State.Dead:

	case container.Container.State.Running:
		// do nothing

	default:
		return nil, fmt.Errorf("unexpected Container.State `%s`", container.Container.State.StateString())
	}

	return container, nil
}

func checkContainerConfig(container *dockertest.Resource, expectedOptions *testContainerOptions) (err error) {
	// image check
	expectImage := expectedOptions.repository + ":" + expectedOptions.imageTag
	if container.Container.Config.Image != expectImage {
		return fmt.Errorf(
			"old container have other image - `%s` instead of `%s`",
			container.Container.Config.Image, expectImage,
		)
	}

	// exposed ports check
	for _, exposedPort := range expectedOptions.exposedPorts {
		_, ok := container.Container.Config.ExposedPorts[docker.Port(exposedPort)]
		if !ok {
			return fmt.Errorf(
				"old container doesn't have exposed port `%s`", exposedPort,
			)
		}
	}

	// [skip env check] // differences can be valid
	// [skip cmd check] // expectedOptions can have empty cmd // differences can be valid?

	return nil
}

func recreateContainer(dockerPool *dockertest.Pool, repository string, options *testContainerOptions) (container *dockertest.Resource, err error) {
	err = dockerPool.RemoveContainerByName(fmt.Sprintf("^%s$", options.containerName))
	if err != nil {
		return nil, fmt.Errorf("failed to dockerPool.RemoveContainerByName: %w", err)
	}

	container, err = createAndStartContainer(dockerPool, repository, options) // FIXME: infinite recursion
	if err != nil {
		return nil, fmt.Errorf("failed to createAndStartContainer: %w", err)
	}

	return container, nil
}

func getApiEndpoints(container *dockertest.Resource) (endpointByPrivatePort map[int]ApiEndpoint, err error) {
	mapping := container.Container.NetworkSettings.PortMappingAPI()
	endpointByPrivatePort = make(map[int]ApiEndpoint, len(mapping))

	// linux
	// access by container ip and private (container) port
	// accessible inside and outside container
	host := container.Container.NetworkSettings.Networks["bridge"].IPAddress // container ip
	getPort := func(apiPort docker.APIPort) int64 { return apiPort.PrivatePort }
	// host = linuxLocalhost

	// crutch: for work in macOS
	// access by macOSLocalhost / docker gateway and public (mapped) port
	// XXX: accessible only outside container
	if runtime.GOOS == macOSName {
		host = macOSLocalhost
		getPort = func(apiPort docker.APIPort) int64 { return apiPort.PublicPort }
	}

	for _, apiPort := range mapping {
		endpointByPrivatePort[int(apiPort.PrivatePort)] = ApiEndpoint{
			IP:   host,
			Port: int(getPort(apiPort)),
		}
	}

	return endpointByPrivatePort, nil
}

func getLocalhost() string {
	if runtime.GOOS == macOSName {
		return macOSLocalhost
	}

	return linuxLocalhost
}
