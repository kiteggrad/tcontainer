package tcontainer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/kiteggrad/freeport"
)

const containerAPIPort = 80

func TestMain(m *testing.M) {
	err := RemoveAll(context.Background())
	if err != nil {
		log.Fatal("failed to RemoveAll:", err)
	}

	goleak.VerifyTestMain(m)
}

// newBusybox - creates minimal configureated busybox container for tests.
func newBusybox(customOpts ...TestContainerOption) (dockerPool *dockertest.Pool, container *dockertest.Resource, err error) {
	startServerCMD := fmt.Sprintf(`echo 'Hello, World!' > /index.html && httpd -p %d -h / && tail -f /dev/null`, containerAPIPort)

	retry := func(container *dockertest.Resource, _ map[int]APIEndpoint) (err error) {
		return pingBusyboxContainerServer(container)
	}

	opts := append([]TestContainerOption{
		WithCMD("sh", "-c", startServerCMD),
		WithExposedPorts(containerAPIPort),
		WithRetry(retry, 0),
	}, customOpts...)

	dockerPool, container, err = New("busybox", opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to New: %w", err)
	}

	return dockerPool, container, nil
}

// pingBusyboxContainerServer - we can use this to check that container is healthy.
func pingBusyboxContainerServer(container *dockertest.Resource) error {
	endpoint := GetAPIEndpoints(container)[containerAPIPort]

	resp, err := http.Get("http://" + endpoint.NetJoinHostPort())
	if err != nil {
		return fmt.Errorf("failed to http.Get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status `%s`", resp.Status)
	}

	return nil
}

func Test_applyTestContainerOptions(t *testing.T) {
	t.Parallel()

	type args struct {
		repository string
		customOpts []TestContainerOption
	}
	type want struct {
		options *testContainerOptions
		err     error
	}
	type testCase struct {
		skip string
		name string
		args args
		want want
	}
	type prepareTestCase func(t *testing.T) testCase
	testCases := []prepareTestCase{
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			return testCase{
				name: "without_custom_args",
				args: args{},
				want: want{options: options},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.imageTag = "v1.2.3"

			return testCase{
				name: "WithImageTag",
				args: args{customOpts: []TestContainerOption{WithImageTag(options.imageTag)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithImageTag_empty",
				args: args{customOpts: []TestContainerOption{WithImageTag("")}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.containerName = "SomeContainerName"

			return testCase{
				name: "WithContainerName",
				args: args{customOpts: []TestContainerOption{WithContainerName(options.containerName)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithContainerName_empty",
				args: args{customOpts: []TestContainerOption{WithContainerName("")}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.containerName = "invalid-between-joined"

			return testCase{
				name: "WithContainerName",
				args: args{customOpts: []TestContainerOption{WithContainerName("invalid/between", "", "", "joined")}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithContainerName_empty",
				args: args{customOpts: []TestContainerOption{WithContainerName("", "")}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.env = []string{"SOME_ENV=some_val"}

			return testCase{
				name: "WithENV",
				args: args{customOpts: []TestContainerOption{WithENV(options.env...)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithENV_empty",
				args: args{customOpts: []TestContainerOption{WithENV()}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithENV_empty_string",
				args: args{customOpts: []TestContainerOption{WithENV("")}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithENV_invalid_format",
				args: args{customOpts: []TestContainerOption{WithENV("kek")}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.cmd = []string{"server", "start"}

			return testCase{
				name: "WithCMD",
				args: args{customOpts: []TestContainerOption{WithCMD(options.cmd...)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithCMD_empty",
				args: args{customOpts: []TestContainerOption{WithCMD()}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.retryOperation = func(_ *dockertest.Resource, _ map[int]APIEndpoint) (_ error) { return nil }
			options.retryTimeout = 123

			return testCase{
				skip: "deepEqual always returns false for functions",
				name: "WithRetry",
				args: args{customOpts: []TestContainerOption{WithRetry(options.retryOperation, options.retryTimeout)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithRetry_empty",
				args: args{customOpts: []TestContainerOption{WithRetry(nil, 123)}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.retryOperation = func(_ *dockertest.Resource, _ map[int]APIEndpoint) (_ error) { return nil }

			return testCase{
				name: "WithRetry_negative_timeout",
				args: args{customOpts: []TestContainerOption{WithRetry(options.retryOperation, -123)}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.exposedPorts = []string{"8080/tcp", "8081/tcp"}

			return testCase{
				name: "WithExposedPorts",
				args: args{customOpts: []TestContainerOption{WithExposedPorts(8080, 8081)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithExposedPorts_empty",
				args: args{customOpts: []TestContainerOption{WithExposedPorts()}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithExposedPorts_negative_port",
				args: args{customOpts: []TestContainerOption{WithExposedPorts(-123)}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.portBindings = map[docker.Port][]docker.PortBinding{
				"80/tcp": {{HostIP: "0.0.0.0", HostPort: "8080"}},
				"82/tcp": {{HostIP: "0.0.0.0", HostPort: "8082"}},
			}

			return testCase{
				name: "WithPortBindings",
				args: args{customOpts: []TestContainerOption{WithPortBindings(map[int]int{80: 8080, 82: 8082})}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithPortBindings_empty",
				args: args{customOpts: []TestContainerOption{WithPortBindings(nil)}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.containerExpiry = 123

			return testCase{
				name: "WithExpiry",
				args: args{customOpts: []TestContainerOption{WithExpiry(options.containerExpiry)}},
				want: want{options: options},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.containerExpiry = 0

			return testCase{
				name: "WithExpiry_empty",
				args: args{customOpts: []TestContainerOption{WithExpiry(options.containerExpiry)}},
				want: want{options: options},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.autoremoveContainer = true

			return testCase{
				name: "WithAutoremove",
				args: args{customOpts: []TestContainerOption{WithAutoremove(options.autoremoveContainer)}},
				want: want{options: options},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.autoremoveContainer = false

			return testCase{
				name: "WithAutoremove_false",
				args: args{customOpts: []TestContainerOption{WithAutoremove(options.autoremoveContainer)}},
				want: want{options: options},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.reuseContainer = true
			options.reuseContainerTimeout = time.Second
			options.reuseContainerRecreateOnErr = true

			return testCase{
				name: "WithReuseContainer",
				args: args{customOpts: []TestContainerOption{WithReuseContainer(true, time.Second, true)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				// we need this case because of backoff initial timeout is second
				name: "WithReuseContainer_too_small_timout",
				args: args{customOpts: []TestContainerOption{WithReuseContainer(true, 1, true)}},
				want: want{err: ErrOptionInvalid},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithReuseContainer_WithRemoveContainerOnExists_conflict",
				args: args{customOpts: []TestContainerOption{WithRemoveContainerOnExists(true), WithReuseContainer(true, time.Second, true)}},
				want: want{err: ErrOptionConflict},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.removeContainerOnExists = true

			return testCase{
				name: "WithRemoveContainerOnExists",
				args: args{customOpts: []TestContainerOption{WithRemoveContainerOnExists(options.removeContainerOnExists)}},
				want: want{options: options},
			}
		},
		func(t *testing.T) testCase {
			require := require.New(t)
			options, err := applyTestContainerOptions("") // get defaults
			require.NoError(err)
			require.NotEmpty(options)

			options.removeContainerOnExists = false

			return testCase{
				name: "WithRemoveContainerOnExists_false",
				args: args{customOpts: []TestContainerOption{WithRemoveContainerOnExists(options.removeContainerOnExists)}},
				want: want{options: options},
			}
		},
		func(_ *testing.T) testCase {
			return testCase{
				name: "WithRemoveContainerOnExists_WithReuseContainer_conflict",
				args: args{customOpts: []TestContainerOption{WithReuseContainer(true, time.Second, true), WithRemoveContainerOnExists(true)}},
				want: want{err: ErrOptionConflict},
			}
		},
	}
	for _, prepareTestCase := range testCases {
		prepareTestCase := prepareTestCase
		t.Run(prepareTestCase(t).name, func(t *testing.T) {
			t.Parallel()
			test := prepareTestCase(t)
			if test.skip != "" {
				t.Skip()
			}
			require := require.New(t)

			options, err := applyTestContainerOptions(test.args.repository, test.args.customOpts...)
			require.ErrorIs(err, test.want.err, err)
			require.Equal(test.want.options, options)
		})
	}
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

func Test_New_WithCMD(t *testing.T) {
	t.Parallel()
	t.Skip("already tested by newBusybox")
}

func Test_New_WithExposedPorts(t *testing.T) {
	t.Parallel()
	t.Skip("already tested by newBusybox")
}

func Test_New_WithPortBindings(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	bindedPort := freeport.MustGet()

	_, container, err := newBusybox(WithPortBindings(map[int]int{containerAPIPort: bindedPort}))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	resp, err := http.Get("http://" + net.JoinHostPort(getLocalhost(), strconv.Itoa(bindedPort)))
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
		skip string `exhaustruct:"optional"`
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
			t.Parallel()

			if test.skip != "" {
				t.Skip(test.skip)
			}
			require := require.New(t)
			assert := assert.New(t)

			startedAt := time.Now()

			// run logic
			currentRetryIteration := 0
			pool, container, err := newBusybox(
				WithRetry(
					func(_ *dockertest.Resource, _ map[int]APIEndpoint) (err error) {
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
			errNoSuchContainer := &docker.NoSuchContainer{}
			if test.want.err == nil && !errors.As(err, &errNoSuchContainer) {
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
		var noSuchContainerErr *docker.NoSuchContainer
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
		var noSuchContainerErr *docker.NoSuchContainer
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
	require.ErrorIs(err, ErrContainerAlreadyExists)
}

func Test_New_WithReuseContainer_true(t *testing.T) {
	t.Parallel()

	type testCase struct {
		skip                string `exhaustruct:"optional"`
		name                string
		invalidateContainer func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource)
	}
	testCases := []testCase{
		{
			name: "Running",
			invalidateContainer: func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource) {
				dcontainer, err := dockerPool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Running)
			},
		},
		{
			name: "Paused",
			invalidateContainer: func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource) {
				require.NoError(dockerPool.Client.PauseContainer(container.Container.ID))
				dcontainer, err := dockerPool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Paused)
			},
		},
		{
			name: "Exited",
			invalidateContainer: func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource) {
				require.NoError(dockerPool.Client.KillContainer(docker.KillContainerOptions{ID: container.Container.ID, Signal: docker.SIGKILL}))
				dcontainer, err := dockerPool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.Equal("exited", dcontainer.State.Status)
			},
		},
		{
			name: "Restarting",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource) {
				require.NoError(dockerPool.Client.RestartContainer(container.Container.ID, 0))
				dcontainer, err := dockerPool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Restarting)
			},
		},
		{
			name: "OOMKilled",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource) {
				dcontainer, err := dockerPool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.OOMKilled)
			},
		},
		{
			name: "Dead",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource) {
				dcontainer, err := dockerPool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Dead)
			},
		},
		{
			name: "RemovalInProgress",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, dockerPool *dockertest.Pool, container *dockertest.Resource) {
				dcontainer, err := dockerPool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.RemovalInProgress)
			},
		},
	}
	for _, test := range testCases {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.skip != "" {
				t.Skip(test.skip)
			}
			require := require.New(t)
			assert := assert.New(t)

			// create container
			dockerPool, container, err := newBusybox(WithContainerName(t.Name()), WithAutoremove(false))
			require.NoError(err)
			t.Cleanup(func() { assert.NoError(container.Close()) })
			containerIDSrc := container.Container.ID

			// invalidate container is it's needed for get different states like "paused"
			test.invalidateContainer(require, dockerPool, container)

			// try reuse container
			_, container, err = newBusybox(WithContainerName(t.Name()), WithReuseContainer(true, 0, false))
			require.NoError(err)
			require.Equal(containerIDSrc, container.Container.ID)  // check we reuse the container
			require.NoError(pingBusyboxContainerServer(container)) // check container is ok
		})
	}
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

	_, container, err = newBusybox(WithContainerName(containerName), WithReuseContainer(true, 0, true), WithExposedPorts(containerAPIPort, freeport.MustGet()))
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

func Test_checkContainerConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		oldContainerOptions []TestContainerOption
		newContainerOptions []TestContainerOption
	}
	tests := []struct {
		skip string
		name string
		args args
		err  error
	}{
		{
			name: "equal",
			args: args{
				oldContainerOptions: []TestContainerOption{},
				newContainerOptions: []TestContainerOption{},
			},
			err: nil,
		},
		{
			name: "image_tag_not_equal",
			args: args{
				oldContainerOptions: []TestContainerOption{WithImageTag("latest")},
				newContainerOptions: []TestContainerOption{WithImageTag("1.36")},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "image_not_equal",
			args: args{
				oldContainerOptions: []TestContainerOption{WithImageTag("latest")},
				newContainerOptions: []TestContainerOption{WithImageTag("latest"), func(options *testContainerOptions) (err error) {
					options.repository = "httpd"
					return nil
				}},
			},
			err: ErrReuseContainerConflict,
		},
		{
			skip: "could be ok",
			name: "env_not_equal",
			args: args{
				oldContainerOptions: []TestContainerOption{},
				newContainerOptions: []TestContainerOption{WithENV("NEQ=neq")},
			},
			err: ErrReuseContainerConflict,
		},
		{
			skip: "could be ok",
			name: "cmd_not_equal",
			args: args{
				oldContainerOptions: []TestContainerOption{},
				newContainerOptions: []TestContainerOption{WithCMD("neq")},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "exposedPorts_not_equal",
			args: args{
				oldContainerOptions: []TestContainerOption{},
				newContainerOptions: []TestContainerOption{WithExposedPorts(9999)},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "portBindings_equal",
			args: args{
				oldContainerOptions: []TestContainerOption{WithPortBindings(
					map[int]int{9999: 9999},
				)},
				newContainerOptions: []TestContainerOption{WithPortBindings(
					map[int]int{9999: 9999},
				)},
			},
			err: nil,
		},
		{
			name: "portBindings_not_equal_port",
			args: args{
				oldContainerOptions: []TestContainerOption{},
				newContainerOptions: []TestContainerOption{WithPortBindings(
					map[int]int{9999: 9999},
				)},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "portBindings_not_equal_port_binding",
			args: args{
				oldContainerOptions: []TestContainerOption{WithPortBindings(
					map[int]int{9999: 9998},
				)},
				newContainerOptions: []TestContainerOption{WithPortBindings(
					map[int]int{9999: 9999},
				)},
			},
			err: ErrReuseContainerConflict,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			require := require.New(t)
			assert := assert.New(t)

			tt.args.oldContainerOptions = append(
				[]TestContainerOption{WithContainerName(t.Name())},
				tt.args.oldContainerOptions...)
			_, oldContainer, err := newBusybox(tt.args.oldContainerOptions...)
			require.NoError(err)
			t.Cleanup(func() { assert.NoError(oldContainer.Close()) })

			tt.args.newContainerOptions = append(
				[]TestContainerOption{WithContainerName(t.Name()), WithReuseContainer(true, 0, false)},
				tt.args.newContainerOptions...)
			_, _, err = newBusybox(tt.args.newContainerOptions...)
			require.ErrorIs(err, tt.err)
		})
	}
}

func Test_RemoveAll(t *testing.T) { //nolint:paralleltest
	require := require.New(t)
	assert := assert.New(t)

	// create containers using this package
	containerName := t.Name()
	dockerPool, container, err := newBusybox(WithContainerName(containerName + "1"))
	require.NoError(err)
	_, container2, err := newBusybox(WithContainerName(containerName + "2"))
	require.NoError(err)

	// create some other container
	_, container3, err := newBusybox(func(options *testContainerOptions) (err error) { options.labels = nil; return nil })
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container3.Close()) })

	err = RemoveAll(context.Background())
	require.NoError(err)

	notRemovedContainers, err := dockerPool.Client.ListContainers(docker.ListContainersOptions{
		All:     true,
		Context: context.Background(),
	})
	require.NoError(err)

	found := false
	for _, notRemovedContainer := range notRemovedContainers {
		require.NotEqual(notRemovedContainer.ID, container.Container.ID)
		require.NotEqual(notRemovedContainer.ID, container2.Container.ID)
		if !found {
			found = notRemovedContainer.ID == container3.Container.ID
		}
	}
	require.True(found, "side container unexpectedly removed")
}
