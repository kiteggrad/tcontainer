package tcontainer

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/kiteggrad/freeport"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

const containerApiPort = 80

// newBusybox - creates minimal configureated busybox container for tests.
func newBusybox(customOpts ...TestContainerOption) (dockerPool *dockertest.Pool, container *dockertest.Resource, err error) {
	startServerCMD := fmt.Sprintf(`echo 'Hello, World!' > /index.html && httpd -p %d -h / && tail -f /dev/null`, containerApiPort)

	retry := func(container *dockertest.Resource, apiEndpoints map[int]ApiEndpoint) (err error) {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d", apiEndpoints[containerApiPort].IP, apiEndpoints[containerApiPort].Port))
		if err != nil {
			return fmt.Errorf("failed to http.Get: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected response status `%s`", resp.Status)
		}

		return nil
	}

	opts := append([]TestContainerOption{
		WithCMD("sh", "-c", startServerCMD),
		WithExposedPorts(containerApiPort),
		WithRetry(retry, 0),
	}, customOpts...)

	dockerPool, container, err = New("busybox", opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to New: %w", err)
	}

	container.Container, err = dockerPool.Client.InspectContainer(container.Container.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to InspectContainer: %w", err)
	}

	return dockerPool, container, err
}

func Test_New_Simple(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	dockerPool, container, err := newBusybox()
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	require.NotEmpty(container)
	require.NotEmpty(dockerPool)
}

func Test_New_WithImageTag(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	const tag = "stable"

	_, container, err := newBusybox(WithImageTag(tag))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	require.Contains(container.Container.Config.Image, ":"+tag)
}

func Test_New_WithContainerName(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	name := t.Name()

	_, container, err := newBusybox(WithContainerName(name))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	require.Contains(container.Container.Name, name)
}

func Test_New_WithENV(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	const env = "KEK=LOL"

	_, container, err := newBusybox(WithENV(env))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	require.Contains(container.Container.Config.Env, env)
}

// already tested by newBusybox
// test WithCMD(cmd ...string)
// test WithExposedPorts(exposedPorts ...int)

func Test_New_WithPortBindings(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	bindedPort := freeport.MustGet()

	_, container, err := newBusybox(WithPortBindings(map[int]int{containerApiPort: bindedPort}))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	resp, err := http.Get(fmt.Sprintf("http://%s:%d", getLocalhost(), bindedPort))
	require.NoError(err)
	defer resp.Body.Close()
	require.Equal(http.StatusOK, resp.StatusCode)
}

func Test_New_WithStartTimeout(t *testing.T) {
	t.Parallel()

	// NOTE: expected that backoff.StartInterval is second

	type args struct {
		startTimeout          time.Duration
		retryIterationSleep   time.Duration
		retryFailedIterations int
	}
	type want struct {
		err error
	}
	type testCase struct {
		skip string
		args args
		want want
	}
	testCases := map[string]testCase{
		"pass/first_iteration": {
			args: args{
				startTimeout:          time.Second,
				retryIterationSleep:   time.Second / 2,
				retryFailedIterations: 0,
			},
			want: want{err: nil},
		},
		"pass/with_failed_iteration": {
			args: args{
				startTimeout:          time.Second * 2,
				retryIterationSleep:   time.Second,
				retryFailedIterations: 1,
			},
			want: want{err: nil},
		},
		"not_pass": {
			args: args{
				startTimeout:          time.Second,
				retryIterationSleep:   time.Second + 1,
				retryFailedIterations: 1,
			},
			want: want{err: ErrRetryTimeout},
		},
		"not_pass/first_iteration": {
			skip: "failed because of retry func always runs at least once",
			args: args{
				startTimeout:          time.Second,
				retryIterationSleep:   time.Second * 2,
				retryFailedIterations: 0,
			},
			want: want{err: ErrRetryTimeout},
		},
	}
	for name, test := range testCases {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			if test.skip != "" {
				t.Skip(test.skip)
			}
			t.Parallel()
			require := require.New(t)
			assert := assert.New(t)

			startedAt := time.Now()

			// run logic
			currentRetryIteration := 0
			pool, container, err := newBusybox(
				WithRetry(
					func(container *dockertest.Resource, apiEndpoints map[int]ApiEndpoint) (err error) {
						defer func() { currentRetryIteration++ }()

						if currentRetryIteration == 0 {
							startedAt = time.Now()
						}

						time.Sleep(test.args.retryIterationSleep)
						if currentRetryIteration < test.args.retryFailedIterations {
							return fmt.Errorf("unexpected interation count %d", currentRetryIteration)
						}

						return nil
					},
					test.args.startTimeout,
				),
			)
			_ = pool
			if test.want.err == nil {
				t.Cleanup(func() { assert.NoError(container.Close()) })
			}
			require.ErrorIs(err, test.want.err, err)

			creatingContainerDuration := time.Since(startedAt)
			require.Less(creatingContainerDuration, test.args.startTimeout+time.Second*2)
		})
	}
}

func Test_New_WithExpiry(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	// assert := assert.New(t)

	const expiry = time.Second

	dockerPool, container, err := newBusybox(WithExpiry(expiry))
	require.NoError(err)
	// t.Cleanup(func() { assert.NoError(container.Close()) })

	container.Container, err = dockerPool.Client.InspectContainer(container.Container.ID)
	require.NoError(err)
	require.True(container.Container.State.Running)

	time.Sleep(expiry + time.Second*2)

	container.Container, err = dockerPool.Client.InspectContainer(container.Container.ID)
	if err != nil {
		noSuchContainerErr := &docker.NoSuchContainer{}
		require.ErrorAs(err, &noSuchContainerErr)
	} else {
		require.False(container.Container.State.Running)
	}
}

func Test_New_WithAutoremove(t *testing.T) {
	t.Parallel()

	const expiry = time.Second * 2

	t.Run("false", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		assert := assert.New(t)

		dockerPool, container, err := newBusybox(WithExpiry(expiry), WithAutoremove(false))
		require.NoError(err)
		t.Cleanup(func() { assert.NoError(container.Close()) })

		container.Container, err = dockerPool.Client.InspectContainer(container.Container.ID)
		require.NoError(err)
		require.True(container.Container.State.Running)

		time.Sleep(expiry + time.Second*2)

		container.Container, err = dockerPool.Client.InspectContainer(container.Container.ID)
		require.NoError(err)
		require.False(container.Container.State.Running)
	})

	t.Run("true", func(t *testing.T) {
		t.Parallel()
		require := require.New(t)
		// assert := assert.New(t)

		dockerPool, container, err := newBusybox(WithExpiry(expiry), WithAutoremove(true))
		require.NoError(err)
		// t.Cleanup(func() { assert.NoError(container.Close()) })

		container.Container, err = dockerPool.Client.InspectContainer(container.Container.ID)
		require.NoError(err)
		require.True(container.Container.State.Running)

		time.Sleep(expiry + time.Second*2)

		container.Container, err = dockerPool.Client.InspectContainer(container.Container.ID)
		noSuchContainerErr := &docker.NoSuchContainer{}
		require.ErrorAs(err, &noSuchContainerErr)
	})
}

func Test_New_WithReuseContainer_false(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	containerName := t.Name()

	_, container, err := newBusybox(WithContainerName(containerName))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	_, _, err = newBusybox(WithContainerName(containerName), WithReuseContainer(false, 0, false))
	require.ErrorIs(err, docker.ErrContainerAlreadyExists)
}

func Test_New_WithReuseContainer_true(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	containerName := t.Name()

	_, container, err := newBusybox(WithContainerName(containerName))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	_, container, err = newBusybox(WithContainerName(containerName), WithReuseContainer(true, 0, false))
	require.NoError(err)
}

func Test_New_WithReuseContainer_recreateOnError(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	containerName := t.Name()

	_, container, err := newBusybox(WithContainerName(containerName))
	require.NoError(err)
	t.Cleanup(func() { _ = container.Close() })

	oldContainerID := container.Container.ID
	assert.NotEmpty(oldContainerID)

	_, container, err = newBusybox(WithContainerName(containerName), WithReuseContainer(true, 0, true), WithExposedPorts(containerApiPort, freeport.MustGet()))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	newContainerID := container.Container.ID
	assert.NotEmpty(newContainerID)
	require.NotEqual(oldContainerID, newContainerID)
}

func Test_New_WithRemoveContainerOnExists(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	containerName := t.Name()

	_, container, err := newBusybox(WithContainerName(containerName))
	require.NoError(err)
	t.Cleanup(func() { _ = container.Close() })

	oldContainerID := container.Container.ID
	assert.NotEmpty(oldContainerID)

	_, container, err = newBusybox(WithContainerName(containerName), WithRemoveContainerOnExists(true))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	newContainerID := container.Container.ID
	assert.NotEmpty(newContainerID)
	require.NotEqual(oldContainerID, newContainerID)
}
