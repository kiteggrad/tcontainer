package tcontainer

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"
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

	labelKeyValue = "tcontainer"
)

var (
	// ErrContainerAlreadyExists - occurs when the container already exists.
	ErrContainerAlreadyExists = docker.ErrContainerAlreadyExists
	// ErrRetryTimeout - occurs when RetryOperation returns errors too long (see retryTimeout in WithRetry()).
	ErrRetryTimeout = errors.New("retry timeout")
	// ErrUnreusableState - occurs when it's impossible to reuse container (see WithReuseContainer()).
	ErrUnreusableState = errors.New("imposible to reuse container with it's current state")
	// ErrReuseContainerConflict - occurs when existed container have different options (e.q. image tag).
	ErrReuseContainerConflict = errors.New("imposible to reuse container, it has differnent options")

	errRepositoryIsRequired = errors.New("repository is required")
)

// Endpoint that you can use to connect to the container.
//
// Note: macOS users may encounter issues accessing the container through APIEndpoint
// from inside the container. This is because macOS users cannot use the container's IP directly,
// potentially leading to connectivity problems.
type APIEndpoint struct {
	IP   string // localhost/dockerGateway or container IP
	Port int    // publicPort or private port
}

// PortStr - get port as string.
func (e APIEndpoint) PortStr() string {
	return strconv.Itoa(e.Port)
}

// NetJoinHostPort - combines ip and port into a network address of the form "host:port".
func (e APIEndpoint) NetJoinHostPort() string {
	return net.JoinHostPort(e.IP, e.PortStr())
}

// RetryOperation is an exponential backoff retry operation. You can use it to wait for e.g. mysql to boot up.
//
// `apiEndpoints` is map that provides you APIEndpoint by each privatePort (port inside the container).
type RetryOperation func(container *dockertest.Resource, apiEndpoints map[int]APIEndpoint) (err error)

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

	return newWithPool(dockerPool, options)
}

func newWithPool(
	dockerPool *dockertest.Pool,
	options *testContainerOptions,
) (
	container *dockertest.Resource,
	err error,
) {
	container, err = initContainer(dockerPool, options)
	if err != nil {
		return nil, fmt.Errorf("failed to initContainer: %w", err)
	}

	if options.containerExpiry != 0 {
		err = container.Expire(uint(options.containerExpiry.Seconds()))
		if err != nil {
			_ = dockerPool.Purge(container)
			return nil, fmt.Errorf("failed to container.Expire: %w", err)
		}
	}

	apiEndpoints := GetAPIEndpoints(container)

	if options.retryOperation != nil {
		retryBackoff := backoff.NewExponentialBackOff()
		retryBackoff.MaxInterval = retryOperationMaxInterval
		retryBackoff.MaxElapsedTime = options.retryTimeout

		err = backoff.Retry(func() (err error) { return options.retryOperation(container, apiEndpoints) }, retryBackoff)
		if err != nil {
			_ = dockerPool.Purge(container)
			if retryBackoff.NextBackOff() == backoff.Stop {
				err = fmt.Errorf("%w: %w", ErrRetryTimeout, err)
			}
			return nil, fmt.Errorf("failed to backoff.Retry: %w", err)
		}
	}

	return container, nil
}

func initContainer(dockerPool *dockertest.Pool, options *testContainerOptions) (
	container *dockertest.Resource, err error,
) {
	if options.repository == "" {
		return nil, errRepositoryIsRequired
	}

	container, err = createAndStartContainer(dockerPool, options)
	switch {
	case err == nil:
		return container, nil

	case errors.Is(err, ErrContainerAlreadyExists) && options.reuseContainer:
		container, err = reuseOrRecreateContainer(dockerPool, options)
		if err != nil {
			return nil, fmt.Errorf("failed to reuseOrRecreateContainer: %w", err)
		}

		return container, nil

	case errors.Is(err, ErrContainerAlreadyExists) && options.removeContainerOnExists:
		container, err := recreateContainer(dockerPool, options)
		if err != nil {
			return nil, fmt.Errorf("failed to recreateContainer by removeContainerOnExists: %w", err)
		}

		return container, nil

	default:
		return nil, fmt.Errorf("failed to createAndStartContainer: %w", err)
	}
}

func createAndStartContainer(dockerPool *dockertest.Pool, options *testContainerOptions) (
	container *dockertest.Resource, err error,
) {
	var auth docker.AuthConfiguration

	// TODO: add aptions for all
	container, err = dockerPool.RunWithOptions(&dockertest.RunOptions{
		Hostname:     "",
		Name:         options.containerName,
		Repository:   options.repository,
		Tag:          options.imageTag,
		Env:          options.env,
		Entrypoint:   nil,
		Cmd:          options.cmd,
		Mounts:       nil,
		Links:        nil,
		ExposedPorts: options.exposedPorts,
		ExtraHosts:   nil,
		CapAdd:       nil,
		SecurityOpt:  nil,
		DNS:          nil,
		WorkingDir:   "",
		NetworkID:    "",
		Networks:     nil,
		Labels:       map[string]string{"tcontainer": "tcontainer"},
		Auth:         auth,
		PortBindings: options.portBindings,
		Privileged:   false,
		User:         "",
		Tty:          false,
		Platform:     "linux/" + runtime.GOARCH,
	}, func(config *docker.HostConfig) {
		config.AutoRemove = options.autoremoveContainer
		config.RestartPolicy = docker.RestartPolicy{Name: "no", MaximumRetryCount: 0}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dockerPool.RunWithOptions: %w", err)
	}

	return container, nil
}

// reuseOrRecreateContainer - try to reuse container, or recreate (optional) if failed to reuse.
func reuseOrRecreateContainer(dockerPool *dockertest.Pool, options *testContainerOptions) (
	container *dockertest.Resource, err error,
) {
	container, err = reuseContainer(dockerPool, options)
	switch {
	case err == nil:
		return container, nil

	case options.reuseContainerRecreateOnErr:
		err = fmt.Errorf("failed to reuseContainer: %w", err)

		container, recreateErr := recreateContainer(dockerPool, options)
		if recreateErr != nil {
			recreateErr = fmt.Errorf("failed to recreateContainer after reuseContainer err: %w", err)
			return nil, errors.Join(err, recreateErr)
		}

		return container, nil

	default:
		return nil, fmt.Errorf("failed to reuseContainer: %w", err)
	}
}

func reuseContainer(dockerPool *dockertest.Pool, options *testContainerOptions) (
	container *dockertest.Resource, err error,
) {
	try := func() (err error) {
		var ok bool
		container, ok = dockerPool.ContainerByName(fmt.Sprintf("^%s$", options.containerName))
		if !ok {
			return backoff.Permanent(fmt.Errorf("failed to dockerPool.ContainerByName `%s`: %w", options.containerName, err))
		}

		err = checkContainerState(container.Container)
		if err != nil {
			return fmt.Errorf("failed to checkContainerState: %w", err)
		}

		err = checkContainerConfig(container.Container, options)
		if err != nil {
			return backoff.Permanent(fmt.Errorf("failed to checkContainerConfig: %w", err))
		}

		return nil
	}

	if try() == nil {
		return container, nil
	}

	err = repairForReuse(dockerPool.Client, container.Container)
	if err != nil {
		return nil, fmt.Errorf("failed to repairForReuse: %w", err)
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = time.Second
	bo.MaxInterval = time.Second
	bo.MaxElapsedTime = options.reuseContainerTimeout

	err = backoff.Retry(try, bo)
	if err != nil {
		return nil, fmt.Errorf("failed to retry after repairForReuse: %w", err)
	}

	return container, nil
}

// repairForReuse - do something to fix container state, do nothing if container is ok.
func repairForReuse(client *docker.Client, container *docker.Container) (err error) {
	switch {
	case checkContainerState(container) == nil:
		return nil

	case container.State.Restarting:
		return nil

	case container.State.Paused:
		err = client.UnpauseContainer(container.ID)
		if err != nil {
			return fmt.Errorf("failed to UnpauseContainer: %w", err)
		}

	case container.State.Status == "exited":
		err = client.StartContainer(container.ID, container.HostConfig)
		if err != nil {
			return fmt.Errorf("failed to StartContainer on `exited` status: %w", err)
		}

	case container.State.OOMKilled, container.State.Dead, container.State.RemovalInProgress:
		return backoff.Permanent(fmt.Errorf("%w: `%s`", ErrUnreusableState, container.State.String())) //nolint:wrapcheck

	default:
		return fmt.Errorf("unexpected Container.State `%s`", container.State.StateString())
	}

	return nil
}

// checkContainerState - checks that container is ready.
func checkContainerState(container *docker.Container) (err error) {
	switch {
	case container.State.Paused:
		return errors.New("still paused")

	case container.State.Status == "exited":
		return errors.New("still exited")

	case container.State.Restarting:
		return errors.New("still restarting")

	case container.State.Running:
		return nil

	case container.State.OOMKilled, container.State.Dead, container.State.RemovalInProgress:
		return backoff.Permanent(fmt.Errorf("%w: %s", ErrUnreusableState, container.State.String())) //nolint:wrapcheck

	default:
		return fmt.Errorf("unexpected Container.State `%s`", container.State.StateString())
	}
}

func checkContainerConfig(container *docker.Container, expectedOptions *testContainerOptions) (err error) {
	// image check
	expectImage := expectedOptions.repository + ":" + expectedOptions.imageTag
	if container.Config.Image != expectImage {
		return fmt.Errorf(
			"%w: other image - `%s` (old) instead of `%s` (new)",
			ErrReuseContainerConflict, container.Config.Image, expectImage,
		)
	}

	// exposed ports check
	for _, exposedPort := range expectedOptions.exposedPorts {
		_, ok := container.Config.ExposedPorts[docker.Port(exposedPort)]
		if !ok {
			return fmt.Errorf(
				"%w: old container doesn't have exposed port `%s`", ErrReuseContainerConflict, exposedPort,
			)
		}
	}

	// port bindings check
	for port, bindings := range expectedOptions.portBindings {
		oldBindings, ok := container.HostConfig.PortBindings[port]
		if !ok {
			return fmt.Errorf(
				"%w: old container doesn't have binding for port `%s`", ErrReuseContainerConflict, port,
			)
		}

		for _, binding := range bindings {
			found := false
			for _, oldBinding := range oldBindings {
				found = oldBinding == binding
				if found {
					break
				}
			}
			if !found {
				return fmt.Errorf(
					"%w: old container doesn't port binding `%#+v` for port `%s`",
					ErrReuseContainerConflict, binding, port,
				)
			}
		}
	}

	// [skip env check] // differences can be valid
	// [skip cmd check] // expectedOptions can have empty cmd // differences can be valid?

	return nil
}

func recreateContainer(dockerPool *dockertest.Pool, options *testContainerOptions) (
	container *dockertest.Resource, err error,
) {
	err = dockerPool.RemoveContainerByName(fmt.Sprintf("^%s$", options.containerName))
	if err != nil {
		return nil, fmt.Errorf("failed to dockerPool.RemoveContainerByName: %w", err)
	}

	container, err = createAndStartContainer(dockerPool, options)
	if err != nil {
		return nil, fmt.Errorf("failed to createAndStartContainer: %w", err)
	}

	return container, nil
}

// GetAPIEndpoints - provides you APIEndpoint by each privatePort (port inside the container).
func GetAPIEndpoints(container *dockertest.Resource) (endpointByPrivatePort map[int]APIEndpoint) {
	mapping := container.Container.NetworkSettings.PortMappingAPI()
	endpointByPrivatePort = make(map[int]APIEndpoint, len(mapping))

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
		endpointByPrivatePort[int(apiPort.PrivatePort)] = APIEndpoint{
			IP:   host,
			Port: int(getPort(apiPort)),
		}
	}

	return endpointByPrivatePort
}

func getLocalhost() string {
	if runtime.GOOS == macOSName {
		return macOSLocalhost
	}

	return linuxLocalhost
}
