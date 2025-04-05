package tcontainer

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strconv"

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
	// ErrContainerAlreadyExists - occurs when the container already exists.
	ErrContainerAlreadyExists = docker.ErrContainerAlreadyExists
	// ErrUnreusableState - occurs when it's impossible to reuse container (see WithReuseContainer()).
	ErrUnreusableState = errors.New("imposible to reuse container with it's current state")
	// ErrReuseContainerConflict - occurs when existed container have different options (e.q. image tag).
	ErrReuseContainerConflict = errors.New("imposible to reuse container, it has differnent options")
)

type (
	// Endpoint that you can use to connect to the container.
	//
	// Note: macOS users may encounter issues accessing the container through APIEndpoint
	// from inside the container. This is because macOS users cannot use the container's IP directly,
	// potentially leading to connectivity problems.
	APIEndpoint struct {
		IP   string // localhost/dockerGateway or container IP
		Port string // publicPort or private port
	}

	// PrivatePort - port inside the container.
	PrivatePort = string

	// RetryOperation is an exponential backoff retry operation. You can use it to wait for e.g. mysql to boot up.
	RetryOperation func(ctx context.Context, container *dockertest.Resource) (err error)
)

// NetJoinHostPort - combines ip and port into a network address of the form "host:port".
func (e APIEndpoint) NetJoinHostPort() string {
	return net.JoinHostPort(e.IP, e.Port)
}

// Pool with docker client.
type Pool struct {
	Pool *dockertest.Pool
}

func NewPool(endpoint string) (Pool, error) {
	pool, err := dockertest.NewPool(endpoint)
	if err != nil {
		return Pool{}, err //nolint:wrapcheck
	}

	return Pool{Pool: pool}, nil
}

func MustNewPool(endpoint string) Pool {
	pool, err := NewPool(endpoint)
	if err != nil {
		panic(err)
	}

	return pool
}

// GetAPIEndpoints - provides you APIEndpoint by each privatePort (port inside the container).
func GetAPIEndpoints(container *dockertest.Resource) (endpointByPrivatePort map[PrivatePort]APIEndpoint) {
	mapping := container.Container.NetworkSettings.PortMappingAPI()
	endpointByPrivatePort = make(map[PrivatePort]APIEndpoint, len(mapping))

	// linux
	// access by container ip and private (container) port
	// accessible inside and outside container
	host := container.Container.NetworkSettings.Networks["bridge"].IPAddress // container ip
	getPort := func(apiPort docker.APIPort) string { return strconv.Itoa(int(apiPort.PrivatePort)) }
	// host = linuxLocalhost

	// crutch: for work in macOS
	// access by macOSLocalhost / docker gateway and public (mapped) port
	// XXX: accessible only outside container
	if runtime.GOOS == macOSName {
		host = macOSLocalhost
		getPort = func(apiPort docker.APIPort) string { return strconv.Itoa(int(apiPort.PublicPort)) }
	}

	for _, apiPort := range mapping {
		endpointByPrivatePort[strconv.Itoa(int(apiPort.PrivatePort))] = APIEndpoint{
			IP:   host,
			Port: getPort(apiPort),
		}
	}

	return endpointByPrivatePort
}

func (p Pool) inspectImageByUUID(ctx context.Context, imageUUID string) (image *docker.Image, err error) {
	foundedImage, err := p.findImageByUUID(ctx, imageUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to findImageByUUID: %w", err)
	}

	return p.Pool.Client.InspectImage(foundedImage.ID) //nolint:wrapcheck
}

func (p Pool) findImageByUUID(ctx context.Context, imageUUID string) (image docker.APIImages, err error) {
	imageList, err := p.Pool.Client.ListImages(docker.ListImagesOptions{
		Filters: map[string][]string{"label": {ImageLabelUUID + "=" + imageUUID}},
		All:     true,
		Digests: true,
		Filter:  "",
		Context: ctx,
	})
	if err != nil {
		return docker.APIImages{}, fmt.Errorf("failed to ListImages: %w", err)
	}

	if len(imageList) == 0 {
		return docker.APIImages{}, errors.New("not found")
	}
	if len(imageList) > 1 {
		return docker.APIImages{}, errors.New("found more than 1 image")
	}

	return imageList[0], nil
}

func ptr[T any](v T) *T {
	return &v
}
