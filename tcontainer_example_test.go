package tcontainer_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	"github.com/kiteggrad/tcontainer"
)

func ExamplePool_Run() {
	const containerAPIPort = "80"
	const serverHelloMesage = "Hello, World!"
	startServerCMD := fmt.Sprintf(`echo '%s' > /index.html && httpd -p %s -h / && tail -f /dev/null`, serverHelloMesage, containerAPIPort)

	// define function to check the server is ready
	url := ""
	pingServerRetry := func(container *dockertest.Resource) (err error) {
		url = "http://" + tcontainer.GetAPIEndpoints(container)[containerAPIPort].NetJoinHostPort()

		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to http.Get: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected response status `%s`", resp.Status)
		}

		return nil
	}

	pool := tcontainer.MustNewPool("")

	// you can remove all containers and images created by this package (from previous tests run)
	// in order to avoid errors like ErrContainerAlreadyExists
	err := pool.Prune(context.Background())
	if err != nil {
		panic(err)
	}

	// run container with the server
	container, err := pool.Run(
		context.Background(),
		"busybox",
		tcontainer.WithContainerName("my-test-server"),
		func(options *tcontainer.RunOptions) (err error) {
			// set by one field instead of *options = tcontainer.RunOptions{...}
			// in order to not owerride default values (like options.Retry.Timeout)

			options.Tag = "latest"
			options.Env = []string{"SOME_ENV=value"}
			options.Cmd = []string{"sh", "-c", startServerCMD}
			options.ExposedPorts = []string{containerAPIPort}
			options.HostConfig.AutoRemove = false
			options.HostConfig.RestartPolicy = docker.RestartPolicy{Name: "no", MaximumRetryCount: 0}
			options.Retry.Operation = pingServerRetry
			options.Reuse.Reuse = true
			options.Reuse.RecreateOnErr = true
			options.ContainerExpiry = time.Minute * 10
			options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
				"80": {{HostIP: "", HostPort: "8080"}},
			}
			options.RemoveOnExists = false // not compatible with Reuse option

			return nil
		},
	)
	if err != nil {
		panic(err)
	}
	defer container.Close() // not necessary if you want to WithReuseContainer

	// make request to the server
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)

	fmt.Println(string(responseBody))

	// Output:
	// Hello, World!
}
