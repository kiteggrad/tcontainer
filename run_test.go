package tcontainer

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kiteggrad/freeport/v2"
)

const containerAPIPort = "80"

// runBusybox - creates minimal configureated busybox container for tests.
func runBusybox(ctx context.Context, customOpts ...RunOption) (pool Pool, container *dockertest.Resource, err error) {
	startServerCMD := fmt.Sprintf(`echo 'Hello, World!' > /index.html && httpd -p %s -h / && tail -f /dev/null`, containerAPIPort)

	opts := append([]RunOption{
		func(options *RunOptions) (err error) {
			options.Cmd = append(options.Cmd, "sh", "-c", startServerCMD)
			options.ExposedPorts = []string{containerAPIPort}
			options.Retry.Operation = pingBusyboxContainerServer
			return nil
		},
	}, customOpts...)

	pool = MustNewPool("")
	container, err = pool.Run(ctx, "busybox", opts...)
	if err != nil {
		return Pool{}, nil, fmt.Errorf("failed to Run: %w", err)
	}

	return pool, container, nil
}

// pingBusyboxContainerServer - we can use this to check that container is healthy.
func pingBusyboxContainerServer(_ context.Context, container *dockertest.Resource) error {
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

func Test_Run(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	pool, container, err := runBusybox(context.Background())
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	require.NotEmpty(container)
	require.NotEmpty(pool)
}

func Test_RunOptions_WithContainerName(t *testing.T) { //nolint:dupl // similar to WithImageName but different
	t.Parallel()

	type args struct {
		nameParts []string
	}
	type want struct {
		name string
	}
	type testCase struct {
		name string
		args args
		want want
	}
	testCases := []testCase{
		{
			name: "camel_case",
			args: args{nameParts: []string{"CamelCase"}},
			want: want{name: "CamelCase"},
		},
		{
			name: "join_parts",
			args: args{nameParts: []string{"1", "2", "3"}},
			want: want{name: "1-2-3"},
		},
		{
			name: "remove_empty_parts",
			args: args{nameParts: []string{"1", "", "3"}},
			want: want{name: "1-3"},
		},
		{
			name: "special_chars",
			args: args{nameParts: []string{"1|2=3_4:5"}},
			want: want{name: "1-2-3_4-5"},
		},
		{
			name: "special_chars/duplicated",
			args: args{nameParts: []string{"1||2=3+4|", "?-5"}},
			want: want{name: "1-2-3-4-5"},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require := require.New(t)

			// run logic
			opts := RunOptions{}
			err := WithContainerName(test.args.nameParts...)(&opts)
			require.NoError(err)
			require.Equal(test.want.name, opts.Name)
		})
	}
}

func Test_RunOptions_ContainerExpiry(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	// assert := assert.New(t)

	const expiry = time.Second

	pool, container, err := runBusybox(context.Background(), func(options *RunOptions) (err error) {
		options.ContainerExpiry = expiry
		return nil
	})
	require.NoError(err)
	// t.Cleanup(func() { assert.NoError(container.Close()) })

	container.Container, err = pool.Pool.Client.InspectContainer(container.Container.ID)
	require.NoError(err)
	require.True(container.Container.State.Running)

	time.Sleep(expiry + time.Second*2)

	container.Container, err = pool.Pool.Client.InspectContainer(container.Container.ID)
	if err != nil {
		var noSuchContainerErr *docker.NoSuchContainer
		require.ErrorAs(err, &noSuchContainerErr)
	} else {
		require.False(container.Container.State.Running)
	}
}

func Test_RunOptions_Reuse_false(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	// create container
	_, container, err := runBusybox(context.Background(), WithContainerName(t.Name()))
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	// create second container with the same name
	_, _, err = runBusybox(context.Background(), WithContainerName(t.Name()), func(options *RunOptions) (err error) {
		options.Reuse.Reuse = false
		return nil
	})
	require.ErrorIs(err, ErrContainerAlreadyExists)
}

func Test_RunOptions_Reuse_true(t *testing.T) {
	t.Parallel()

	type testCase struct {
		skip                string `exhaustruct:"optional"`
		name                string
		invalidateContainer func(require *require.Assertions, pool Pool, container *dockertest.Resource)
	}
	testCases := []testCase{
		{
			name: "Running",
			invalidateContainer: func(require *require.Assertions, pool Pool, container *dockertest.Resource) {
				dcontainer, err := pool.Pool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Running)
			},
		},
		{
			name: "Paused",
			invalidateContainer: func(require *require.Assertions, pool Pool, container *dockertest.Resource) {
				require.NoError(pool.Pool.Client.PauseContainer(container.Container.ID))
				dcontainer, err := pool.Pool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Paused)
			},
		},
		{
			name: "Exited",
			invalidateContainer: func(require *require.Assertions, pool Pool, container *dockertest.Resource) {
				require.NoError(pool.Pool.Client.KillContainer(docker.KillContainerOptions{ID: container.Container.ID, Signal: docker.SIGKILL}))
				dcontainer, err := pool.Pool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.Equal("exited", dcontainer.State.Status)
			},
		},
		{
			name: "Restarting",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, pool Pool, container *dockertest.Resource) {
				require.NoError(pool.Pool.Client.RestartContainer(container.Container.ID, 0))
				dcontainer, err := pool.Pool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Restarting)
			},
		},
		{
			name: "OOMKilled",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, pool Pool, container *dockertest.Resource) {
				dcontainer, err := pool.Pool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.OOMKilled)
			},
		},
		{
			name: "Dead",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, pool Pool, container *dockertest.Resource) {
				dcontainer, err := pool.Pool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.Dead)
			},
		},
		{
			name: "RemovalInProgress",
			skip: "i don't know how to write stable test for this case",
			invalidateContainer: func(require *require.Assertions, pool Pool, container *dockertest.Resource) {
				dcontainer, err := pool.Pool.Client.InspectContainer(container.Container.ID)
				require.NoError(err)
				require.True(dcontainer.State.RemovalInProgress)
			},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if test.skip != "" {
				t.Skip(test.skip)
			}
			require := require.New(t)
			assert := assert.New(t)

			// create container
			pool, container, err := runBusybox(
				context.Background(),
				WithContainerName(t.Name()),
				func(options *RunOptions) (err error) {
					options.HostConfig.AutoRemove = false
					return nil
				},
			)
			require.NoError(err)
			t.Cleanup(func() { assert.NoError(container.Close()) })
			containerIDSrc := container.Container.ID

			// invalidate container is it's needed for get different states like "paused"
			test.invalidateContainer(require, pool, container)

			// try reuse container
			_, container, err = runBusybox(
				context.Background(),
				WithContainerName(t.Name()),
				func(options *RunOptions) (err error) {
					options.Reuse.Reuse = true
					return nil
				})
			require.NoError(err)
			require.Equal(containerIDSrc, container.Container.ID)               // check we reuse the container
			require.NoError(pingBusyboxContainerServer(t.Context(), container)) // check container is ok
		})
	}
}

func Test_RunOptions_Reuse_RecreateOnError(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	// create container
	_, container, err := runBusybox(context.Background(), WithContainerName(t.Name()))
	require.NoError(err)
	t.Cleanup(func() { _ = container.Close() })

	oldContainerID := container.Container.ID
	assert.NotEmpty(oldContainerID)

	// try to reuse container by name
	_, container, err = runBusybox(context.Background(), WithContainerName(t.Name()), func(options *RunOptions) (err error) {
		options.Reuse.Reuse = true
		options.Reuse.RecreateOnErr = true
		options.ExposedPorts = []string{containerAPIPort, freeport.MustGet().String()}
		return nil
	})
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	// check container was recreated
	newContainerID := container.Container.ID
	assert.NotEmpty(newContainerID)
	require.NotEqual(oldContainerID, newContainerID)
}

func Test_RunOptions_RemoveOnExists(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	assert := assert.New(t)

	// create first container
	_, container, err := runBusybox(context.Background(), WithContainerName(t.Name()))
	require.NoError(err)
	t.Cleanup(func() { _ = container.Close() })

	oldContainerID := container.Container.ID
	assert.NotEmpty(oldContainerID)

	// create second container with the same name
	_, container, err = runBusybox(context.Background(), WithContainerName(t.Name()), func(options *RunOptions) (err error) {
		options.RemoveOnExists = true
		return nil
	})
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container.Close()) })

	// check that second runBusybox returns other container id
	newContainerID := container.Container.ID
	assert.NotEmpty(newContainerID)
	require.NotEqual(oldContainerID, newContainerID)
}

func Test_checkContainerConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		oldContainerOptions []RunOption
		newContainerOptions []RunOption
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
				oldContainerOptions: []RunOption{},
				newContainerOptions: []RunOption{},
			},
			err: nil,
		},
		{
			name: "image_tag_not_equal",
			args: args{
				oldContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.Tag = defaultImageTag
					return nil
				}},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.Tag = "1.36"
					return nil
				}},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "image_not_equal",
			args: args{
				oldContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.Tag = defaultImageTag
					return nil
				}},
				newContainerOptions: []RunOption{
					func(options *RunOptions) (err error) {
						options.Tag = defaultImageTag
						options.Repository = "httpd"
						return nil
					},
				},
			},
			err: ErrReuseContainerConflict,
		},
		{
			skip: "could be ok",
			name: "env_not_equal",
			args: args{
				oldContainerOptions: []RunOption{},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.Env = []string{"NEQ=neq"}
					return nil
				}},
			},
			err: ErrReuseContainerConflict,
		},
		{
			skip: "could be ok",
			name: "cmd_not_equal",
			args: args{
				oldContainerOptions: []RunOption{},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.Cmd = []string{"neq"}
					return nil
				}},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "exposedPorts_not_equal",
			args: args{
				oldContainerOptions: []RunOption{},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.ExposedPorts = []string{"9999"}
					return nil
				}},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "portBindings_equal",
			args: args{
				oldContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
						"9999": {{HostPort: "9998"}, {HostPort: "9999"}},
					}
					return nil
				}},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
						"9999": {{HostPort: "9999"}},
					}
					return nil
				}},
			},
			err: nil,
		},
		{
			name: "portBindings_not_equal_port",
			args: args{
				oldContainerOptions: []RunOption{},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
						"9999": {{HostPort: "9999"}},
					}
					return nil
				}},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "portBindings_not_equal_port_binding",
			args: args{
				oldContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
						"9999": {{HostPort: "9998"}},
					}
					return nil
				}},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
						"9999": {{HostPort: "9999"}},
					}
					return nil
				}},
			},
			err: ErrReuseContainerConflict,
		},
		{
			name: "portBindings_not_equal_port_binding_2",
			args: args{
				oldContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
						"9999": {{HostPort: "9998"}, {HostPort: "9999"}},
					}
					return nil
				}},
				newContainerOptions: []RunOption{func(options *RunOptions) (err error) {
					options.HostConfig.PortBindings = map[docker.Port][]docker.PortBinding{
						"9999": {{HostPort: "9998"}},
					}
					return nil
				}},
			},
			err: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			require := require.New(t)
			assert := assert.New(t)

			tt.args.oldContainerOptions = append(
				[]RunOption{WithContainerName(t.Name())},
				tt.args.oldContainerOptions...)
			_, oldContainer, err := runBusybox(context.Background(), tt.args.oldContainerOptions...)
			require.NoError(err)
			t.Cleanup(func() { assert.NoError(oldContainer.Close()) })

			tt.args.newContainerOptions = append(
				[]RunOption{
					WithContainerName(t.Name()),
					func(options *RunOptions) (err error) {
						options.Reuse.Reuse = true
						options.Reuse.RecreateOnErr = false
						return nil
					},
				},
				tt.args.newContainerOptions...)
			_, _, err = runBusybox(context.Background(), tt.args.newContainerOptions...)
			require.ErrorIs(err, tt.err)
		})
	}
}
