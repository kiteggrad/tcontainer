package tcontainer_test

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ory/dockertest/v3"

	"github.com/kiteggrad/tcontainer"
)

func ExampleNew() {
	const containerAPIPort = 80
	const serverHelloMesage = "Hello, World!"
	startServerCMD := fmt.Sprintf(`echo '%s' > /index.html && httpd -p %d -h / && tail -f /dev/null`, serverHelloMesage, containerAPIPort)

	// define function to check the server is ready
	url := ""
	retry := func(_ *dockertest.Resource, apiEndpoints map[int]tcontainer.APIEndpoint) (err error) {
		url = "http://" + apiEndpoints[containerAPIPort].NetJoinHostPort()

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

	// run container with the server
	dockerPool, container, err := tcontainer.New(
		"busybox",
		tcontainer.WithImageTag("latest"),
		tcontainer.WithContainerName("my-test-server"),
		tcontainer.WithENV("SOME_ENV=value"),
		tcontainer.WithCMD("sh", "-c", startServerCMD),
		tcontainer.WithExposedPorts(containerAPIPort),
		tcontainer.WithRetry(retry, 0),               // 0 - defailt timeout
		tcontainer.WithReuseContainer(true, 0, true), // reuseContainer, reuseTimeout, recreateOnError
		tcontainer.WithAutoremove(false),
		tcontainer.WithExpiry(time.Minute*10),
		tcontainer.WithPortBindings(map[int]int{80: 8080}),
		tcontainer.WithRemoveContainerOnExists(false), // not compatible with WithReuseContainer option
	)
	if err != nil {
		panic(err)
	}
	defer container.Close() // not necessary if you want to WithReuseContainer
	_ = dockerPool

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
