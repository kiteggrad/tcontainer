package tcontainer

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	DefaultLabelKeyValue = "tcontainer"

	defaultImageTag                = "latest"
	defaultContainerExpiry         = time.Minute
	defaultAutoremoveContainer     = true
	defaultRemoveContainerOnExists = false

	defaultReuseContainer              = false
	defaultReuseContainerRecreateOnErr = false
	defaultReuseBackoffInitialInterval = time.Second
	defaultReuseBackoffMaxInterval     = time.Second

	defaultRetryBackoffMaxInterval = time.Second * 5
)

var (
	// ErrInvalidOptions - occurs when invalid value was passed to TestContainerOption.
	ErrInvalidOptions = errors.New("invalid option")
	// ErrOptionConflict - occurs when incompatible TestContainerOption have been passed.
	ErrOptionConflict = errors.New("conflicted options")

	containerNameInvalidCharsRegexp = regexp.MustCompile("[^a-zA-Z0-9_.-]")
)

type (
	// RunOptions for (Pool).Run function.
	RunOptions struct {
		Hostname     string
		Name         string
		Repository   string
		Tag          string
		Env          []string
		Entrypoint   []string
		Cmd          []string
		ExposedPorts []string
		WorkingDir   string
		Networks     []*dockertest.Network // optional networks to join
		Labels       map[string]string
		Auth         docker.AuthConfiguration
		User         string
		Tty          bool
		Platform     string
		HostConfig   docker.HostConfig

		// Allows you to reuse a container instead of getting an error that the container already exists.
		// See [RetryOptions] struct description
		Retry           RetryOptions
		ContainerExpiry time.Duration

		// Try to reuse container if it already exists.
		// See [ReuseContainerOptions] struct description.
		Reuse ReuseContainerOptions

		// Allows you to remove an existing container instead of getting an error that the container already exists.
		//	- Should not be used together with `Reuse` - will return `ErrOptionsConflict` error.
		//
		// Default: `false`
		RemoveOnExists bool
	}

	// Allows you to reuse a container instead of getting an error that the container already exists.
	//	- Should not be used together with `RemoveOnExists` - will return `ErrOptionsConflict` error.
	//	- You may get an error if the existing container has different settings (different port mapping or image name).
	//		This error can be ignored with `RecreateOnErr`
	//	- You can specify `Backoff` to change the timeout waiting for the old container to be unpaused or started.
	//	- You can specify `RecreateOnErr` to recreate the container instead of getting an error when trying to reuse it.
	//		(When the old container has different settings or could not be revived)
	//	- Use `ConfigChecks` to check that old container suits for reuse
	//
	// # Default:
	//	- `Reuse` - `false`
	//	- `RecreateOnErr` - `false`
	//	- `ConfigChecks` - checks that old container have the same image, exposed ports and port bindings
	//
	// # Example
	//	func(options *RunOptions) (err error) {
	//		options.Reuse.Reuse = true
	//		reuseBackoff := backoff.NewExponentialBackOff()
	//		reuseBackoff.MaxInterval = time.Second
	//		reuseBackoff.Reset()
	//		options.Reuse.Backoff = reuseBackoff
	//		options.Reuse.ConfigChecks = append(options.Reuse.ConfigChecks,
	//			func(container *docker.Container, expectedOptions RunOptions) (err error) {
	//				if container.Config.Image != expectedOptions.Repository + ":" + expectedOptions.Tag {
	//					return errors.New("old container have other image")
	//				}
	//				return nil
	//			},
	//		)
	//		return nil
	//	}
	ReuseContainerOptions struct {
		Reuse         bool
		Backoff       backoff.BackOff
		RecreateOnErr bool
		ConfigChecks  []ContainerConfigCheck
	}

	// Function for check that container suits for reuse.
	ContainerConfigCheck func(container *docker.Container, expectedOptions RunOptions) (err error)

	// Allows you to specify a command that checks that the container is successfully started and ready to work.
	//	- `Run` function will periodically run and wait for the successful completion of `Retry.Operation`
	//		or issue an error upon reaching `backoff.Stop` / `backoff.Permanent`.
	//	- Use `GetAPIEndpoints(container)` to get the externally accessible ip and port
	//		to connect to a specific internal port of the container.
	//
	// # Default:
	//	- if `Retry.Operation` is not performed, `Run` function complete immediately after container creation
	//
	// # Example:
	//	func(options *RunOptions) (err error) {
	//	    options.Retry.Operation = func(ctx context.Context, container *dockertest.Resource) (err error) {
	//	        fmt.Println("ping")
	//	        return nil
	//	    }
	//	    retryBackoff := backoff.NewExponentialBackOff()
	//	    retryBackoff.MaxInterval = time.Second
	//	    retryBackoff.Reset()
	//	    options.Retry.Backoff = retryBackoff
	//	    return nil
	//	}
	RetryOptions struct {
		Operation RetryOperation
		Backoff   backoff.BackOff
	}

	// RunOption - option for (Pool).Run function.
	// See [ApplyRunOptions].
	RunOption func(options *RunOptions) (err error)
)

// WithContainerName - use custom container name instead of random (generated by docker).
// All invalid characters will be repaced to "-".
// Not empty containerNameParts will be joined with "-" separator, empty parts will be removed.
//
// Example usage:
//
//	WithContainerName(t.Name(), "redis") // "Test/with/invalid/chars", "redis" -> "Test-with-invalid-chars-redis"
func WithContainerName(nameParts ...string) RunOption {
	return func(options *RunOptions) (err error) {
		const delimiter = "-"

		// remove empty parts
		nameParts = slices.DeleteFunc(nameParts, func(s string) bool { return s == "" })

		// join parts
		name := strings.Join(nameParts, delimiter)

		// replace invalid chars
		name = containerNameInvalidCharsRegexp.ReplaceAllString(name, delimiter)

		// replace delimiter duplications
		for strings.Contains(name, delimiter+delimiter) {
			name = strings.ReplaceAll(name, delimiter+delimiter, delimiter)
		}

		// set option
		options.Name = name

		return nil
	}
}

// ApplyRunOptions sets defaults and apply custom options.
// Options aplies in order they passed.
//
// Each option rewrites previous value
//
//	ApplyRunOptions(WithContainerName("first"), WithContainerName("second")) // "second"
func ApplyRunOptions(repository string, customOpts ...RunOption) (
	options RunOptions, err error,
) {
	options = options.getDefault(repository)

	for _, customOpt := range customOpts {
		err = customOpt(&options)
		if err != nil {
			return RunOptions{}, err
		}
	}

	options.Retry.Backoff.Reset()
	options.Reuse.Backoff.Reset()

	err = options.validate()
	if err != nil {
		return RunOptions{}, fmt.Errorf("failed to options.validate: %w", err)
	}

	return options, nil
}

func (o RunOptions) getDefault(repository string) (defaultRunOptions RunOptions) {
	retryBackoff := backoff.NewExponentialBackOff()
	retryBackoff.MaxInterval = defaultRetryBackoffMaxInterval
	retryBackoff.Reset()

	reuseBackoff := backoff.NewExponentialBackOff()
	reuseBackoff.InitialInterval = defaultReuseBackoffInitialInterval
	reuseBackoff.MaxInterval = defaultReuseBackoffMaxInterval
	reuseBackoff.Reset()

	return RunOptions{
		Hostname:     "",
		Name:         "",
		Repository:   repository,
		Tag:          defaultImageTag,
		Env:          nil,
		Entrypoint:   nil,
		Cmd:          nil,
		ExposedPorts: nil,
		WorkingDir:   "",
		Networks:     nil,
		Labels:       map[string]string{DefaultLabelKeyValue: DefaultLabelKeyValue},
		Auth:         docker.AuthConfiguration{}, //nolint:exhaustruct
		User:         "",
		Tty:          false,
		Platform:     "",
		HostConfig: docker.HostConfig{ //nolint:exhaustruct
			AutoRemove: defaultAutoremoveContainer,
		},
		Retry: RetryOptions{
			Operation: nil,
			Backoff:   retryBackoff,
		},
		ContainerExpiry: defaultContainerExpiry,
		Reuse: ReuseContainerOptions{
			Reuse:         defaultReuseContainer,
			Backoff:       reuseBackoff,
			RecreateOnErr: defaultReuseContainerRecreateOnErr,
			ConfigChecks: []ContainerConfigCheck{
				defaultContainerConfigCheck,
			},
		},
		RemoveOnExists: defaultRemoveContainerOnExists,
	}
}

func defaultContainerConfigCheck(container *docker.Container, expectedOptions RunOptions) (err error) {
	// image check
	expectImage := expectedOptions.Repository + ":" + expectedOptions.Tag
	if container.Config.Image != expectImage {
		return fmt.Errorf(
			"other image - `%s` (old) instead of `%s` (new)",
			container.Config.Image, expectImage,
		)
	}

	// exposed ports check
	for _, exposedPort := range expectedOptions.ExposedPorts {
		_, ok := container.Config.ExposedPorts[docker.Port(exposedPort)]
		if !ok {
			return fmt.Errorf(
				"old container doesn't have exposed port `%s`", exposedPort,
			)
		}
	}

	// port bindings check
	err = checkPortBindings(expectedOptions.HostConfig.PortBindings, container.HostConfig.PortBindings)
	if err != nil {
		return fmt.Errorf("failed to checkPortBindings: %w", err)
	}

	// [skip env check] // differences can be valid
	// [skip cmd check] // expectedOptions can have empty cmd // differences can be valid?

	return nil
}

func checkPortBindings(expected, actual map[docker.Port][]docker.PortBinding) (err error) {
	for port, expectedBindings := range expected {
		actualBindings, ok := actual[port]
		if !ok {
			return fmt.Errorf(
				"%w: not found binding for port `%s`", ErrReuseContainerConflict, port,
			)
		}

		for _, expectedBinding := range expectedBindings {
			found := false
			for _, actualBinding := range actualBindings {
				found = actualBinding == expectedBinding
				if found {
					break
				}
			}
			if !found {
				return fmt.Errorf(
					"%w: not found port binding `%#+v` for port `%s`",
					ErrReuseContainerConflict, expectedBinding, port,
				)
			}
		}
	}

	return nil
}

func (o RunOptions) validate() (err error) {
	if o.Repository == "" {
		return fmt.Errorf("%w: repository is required", ErrInvalidOptions)
	}

	if o.RemoveOnExists && o.Reuse.Reuse {
		return fmt.Errorf("%w: RemoveOnExists conflicts with Reuse", ErrOptionConflict)
	}

	return nil
}

func (o RunOptions) toDockertest() (dockertestRunOptions *dockertest.RunOptions) {
	return &dockertest.RunOptions{
		Hostname:     o.Hostname,
		Name:         o.Name,
		Repository:   o.Repository,
		Tag:          o.Tag,
		Env:          o.Env,
		Entrypoint:   o.Entrypoint,
		Cmd:          o.Cmd,
		Mounts:       o.HostConfig.Binds,
		Links:        o.HostConfig.Links,
		ExposedPorts: o.ExposedPorts,
		ExtraHosts:   o.HostConfig.ExtraHosts,
		CapAdd:       o.HostConfig.CapAdd,
		SecurityOpt:  o.HostConfig.SecurityOpt,
		DNS:          o.HostConfig.DNS,
		WorkingDir:   o.WorkingDir,
		NetworkID:    "",
		Networks:     o.Networks,
		Labels:       o.Labels,
		Auth:         o.Auth,
		PortBindings: o.HostConfig.PortBindings,
		Privileged:   o.HostConfig.Privileged,
		User:         o.User,
		Tty:          o.Tty,
		Platform:     o.Platform,
	}
}
