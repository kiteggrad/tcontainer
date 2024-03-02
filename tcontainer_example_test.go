package tcontainer_test

import (
	"errors"
	"time"

	"github.com/kiteggrad/tcontainer"
	"github.com/ory/dockertest/v3"
)

func ExampleNew() {
	connectToDB := func(host string, port int, user, password string) (err error) {
		_, _, _, _ = host, port, user, password
		return errors.New("unimplemented")
	}

	dockerPool, container, err := tcontainer.New(
		"postgres",
		tcontainer.WithImageTag("latest"),
		tcontainer.WithRetry(
			func(container *dockertest.Resource, apiEndpoints map[int]tcontainer.ApiEndpoint) (err error) {
				return connectToDB(apiEndpoints[5432].IP, apiEndpoints[5432].Port, "user", "pass")
			},
			0, // use default retry timeout
		),
		tcontainer.WithReuseContainer(true, 0, true), // reuseContainer, reuseTimeout, recreateOnError
		tcontainer.WithAutoremove(false),
		tcontainer.WithExpiry(time.Minute*10),
	)
	if err != nil {
		panic(err)
	}
	defer func() { _ = container.Close() }()

	_ = dockerPool
	_ = container
}
