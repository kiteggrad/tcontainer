# tcontainer

[![ru](https://img.shields.io/badge/lang-ru-green.svg)](README.ru.md)

Wrapper over https://github.com/ory/dockertest

Provides additional conveniences for creating docker containers in tests:
- more convenient syntax for creating containers using options
- ability to reuse a container if it already exists `WithReuseContainer(...)`
- ability to remove old container when creating a new one instead of getting `docker.ErrContainerAlreadyExists` error

## Usage example

```go
package main

import (
	"errors"
	"time"

	"github.com/kiteggrad/tcontainer"
	"github.com/ory/dockertest/v3"
)

func main() {
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
```

## List of options
- ### `WithImageTag`
    Allows you to specify your own tag for the image

    Default: `latest`

    ```go
    tcontainer.WithImageTag("v1.0.0")
    ```
- ### `WithContainerName`
    Allows you to specify your own name for the container

    Default: docker generates a random name

    ```go
    tcontainer.WithContainerName("test-postgres")
    // or
    tcontainer.WithContainerName(t.Name()+"-pg")
    ```
- ### `WithENV`
    Allows you to pass a set of env variables to the container

    ```go
    tcontainer.WithENV("USER=root", "PASSWORD=password")
    ```
- ### `WithCMD`
    Allows you to specify a command to start the container

    Default: the one specified in the image's Dockerfile is executed

    ```go
    tcontainer.WithCMD("sh", "-c", "server start")
    ```
- ### `WithRetry`
    Allows you to specify a command that checks that the container is successfully started and ready to work.
    - `New` / `NewWithPool` functions will periodically run and wait for the successful completion of `retryOperation`
    or issue an error upon reaching `retryTimeout`.  
    - `apiEndpoints` allows you to get the externally accessible ip and port to connect to a specific internal port of the container.

    Default: 
    - `retryOperation` is not performed, `New` / `NewWithPool` functions complete immediately after container creation 
    - `retryTimeout` - time.Minute

    ```go
    tcontainer.WithRetry(
        func(container *dockertest.Resource, apiEndpoints map[int]tcontainer.ApiEndpoint) (err error) {
            return connectToDB(apiEndpoints[5432].IP, apiEndpoints[5432].Port, "user", "pass")
        },
        0, // use default retry timeout
	)
    ```
- ### `WithExposedPorts`
    Allows you to specify ports to publish. Similar to EXPOSE in Dockerfile

    Currently, all specified ports will be published in tcp (8080/tcp). <!-- //TODO: implement -->
    In the future, it may be possible to specify a different protocol.

    Default: nothing is published

    ```go
    tcontainer.WithExposedPorts(8080, 8081)
    ```
- ### `WithPortBindings`
    Allows you to specify the binding of private ports to specific public ports - `map[privatePort]publicPort`

    Currently, all specified ports will be considered in tcp (8080/tcp). <!-- //TODO: implement -->
    In the future, it may be possible to specify a different protocol.

    Default: all public ports are generated randomly

    ```go
    tcontainer.WithPortBindings(map[int]int{80: 8080, 443: 8443})
    ```
- ### `WithExpiry`
    Allows you to specify the time after which the container will be stopped.
    You can specify an empty value, then the container will not be stopped after any time.

    Default: `time.Minute`

    ```go
    tcontainer.WithExpiry(time.Minute * 10)
    // or
    tcontainer.WithExpiry(0)
    ```
- ### `WithAutoremove`
    Allows you to specify whether the container will be removed immediately after it is stopped (including by expiry).

    Default: `true`

    ```go
    tcontainer.WithAutoremove(false)
    ```
- ### `WithReuseContainer`
    Allows you to reuse a container instead of getting an error that the container already exists.
    - Should not be used together with `WithRemoveContainerOnExists` - will return `ErrOptionsConflict` error.
    - You may get an error if the existing container has different settings (different port mapping or image name). This error can be ignored with `recreateOnError`
    - You can specify `reuseTimeout` to change the timeout waiting for the old container to be unpaused or started.
    - You can specify `recreateOnError` to recreate the container instead of getting an error when trying to reuse it. - When the old container has different settings or could not be revived

    Default: 
    - `reuseContainer` - `false`
    - `reuseTimeout` - `time.Minute`
    - `recreateOnError` - `false`

    ```go
    tcontainer.WithReuseContainer(true, 0, true)
    ```
- ### `WithRemoveContainerOnExists`
    Allows you to remove an existing container instead of getting an error that the container already exists.
    - Should not be used together with `WithRemoveContainerOnExists` - will return `ErrOptionsConflict` error.

    Default: `false`

    ```go
    tcontainer.WithRemoveContainerOnExists(true)
    ```