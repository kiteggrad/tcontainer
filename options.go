package tcontainer

import (
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	defaultImageTag                    = "latest"
	defaultContainerExpiry             = time.Minute
	defaultRetryTimeout                = time.Second * 20 // we use 20s because of 30s is default go test timeout
	defaultAutoremoveContainer         = true
	defaultReuseContainer              = false
	defaultReuseContainerTimeout       = time.Second * 20 // we use 20s because of 30s is default go test timeout
	defaultReuseContainerRecreateOnErr = false
	defaultRemoveContainerOnExists     = false
	defaultLabelKeyValue               = "tcontainer"

	retryOperationMaxInterval = time.Second * 5
)

var (
	// ErrOptionInvalid - occurs when invalid value was passed to TestContainerOption.
	ErrOptionInvalid = errors.New("invalid option")
	// ErrOptionConflict - occurs when incompatible TestContainerOption have been passed.
	ErrOptionConflict = errors.New("conflicted options")

	containerNameInvalidCharsRegexp = regexp.MustCompile("[^a-zA-Z0-9_.-]")
)

// TestContainerOption - option for New() / NewWithPool().
// Options aplies in order they passed.
//
// Each option rewrites previous value
//
//	[]TestContainerOption{WithImageTag("v0.0.1"), WithImageTag("v0.0.2")} // v0.0.2
type TestContainerOption func(options *testContainerOptions) (err error)

type (
	privatePort = int
	publicPort  = int
)

type testContainerOptions struct {
	repository                  string
	imageTag                    string
	containerName               string
	labels                      map[string]string
	env                         []string
	cmd                         []string
	retryOperation              RetryOperation
	retryTimeout                time.Duration
	exposedPorts                []string
	portBindings                map[docker.Port][]docker.PortBinding
	containerExpiry             time.Duration
	autoremoveContainer         bool
	reuseContainer              bool
	reuseContainerTimeout       time.Duration
	reuseContainerRecreateOnErr bool
	removeContainerOnExists     bool
}

func applyTestContainerOptions(repository string, customOpts ...TestContainerOption) (
	opts *testContainerOptions, err error,
) {
	opts = &testContainerOptions{
		repository:                  repository,
		imageTag:                    defaultImageTag,
		containerName:               "",
		labels:                      map[string]string{defaultLabelKeyValue: defaultLabelKeyValue},
		env:                         nil,
		cmd:                         nil,
		retryOperation:              nil,
		retryTimeout:                defaultRetryTimeout,
		exposedPorts:                nil,
		portBindings:                nil,
		containerExpiry:             defaultContainerExpiry,
		autoremoveContainer:         defaultAutoremoveContainer,
		reuseContainer:              defaultReuseContainer,
		reuseContainerTimeout:       defaultReuseContainerTimeout,
		reuseContainerRecreateOnErr: defaultReuseContainerRecreateOnErr,
		removeContainerOnExists:     defaultRemoveContainerOnExists,
	}

	for _, customOpt := range customOpts {
		err = customOpt(opts)
		if err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func (o testContainerOptions) convertToDockertest() (
	runOpts *dockertest.RunOptions, hostConfigOpts []func(*docker.HostConfig),
) {
	var auth docker.AuthConfiguration

	// TODO: add aptions for all
	runOpts = &dockertest.RunOptions{
		Hostname:     "",
		Name:         o.containerName,
		Repository:   o.repository,
		Tag:          o.imageTag,
		Env:          o.env,
		Entrypoint:   nil,
		Cmd:          o.cmd,
		Mounts:       nil,
		Links:        nil,
		ExposedPorts: o.exposedPorts,
		ExtraHosts:   nil,
		CapAdd:       nil,
		SecurityOpt:  nil,
		DNS:          nil,
		WorkingDir:   "",
		NetworkID:    "",
		Networks:     nil,
		Labels:       o.labels,
		Auth:         auth,
		PortBindings: o.portBindings,
		Privileged:   false,
		User:         "",
		Tty:          false,
		Platform:     "linux/" + runtime.GOARCH,
	}
	hostConfigOpts = append(hostConfigOpts, func(config *docker.HostConfig) {
		config.AutoRemove = o.autoremoveContainer
		config.RestartPolicy = docker.RestartPolicy{Name: "no", MaximumRetryCount: 0}
	})

	return runOpts, hostConfigOpts
}

// WithImageTag - use custom image tag instead of default (latest).
//
// Example usage:
//
//	WithImageTag("v1.0.0")
func WithImageTag(imageTag string) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if imageTag == "" {
			return fmt.Errorf("%w: imageTag must not be empty", ErrOptionInvalid)
		}

		options.imageTag = imageTag

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithImageTag with value `%s`: %w", imageTag, err)
		}

		return nil
	}

	return optionWrap
}

// WithContainerName - use custom container name instead of random (generated by docker).
//
// Example usage:
//
//	WithContainerName("project-test-container")
func WithContainerName(containerName string) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if containerName == "" {
			return fmt.Errorf("%w: containerName must not be empty", ErrOptionInvalid)
		}

		options.containerName = containerName

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithContainerName with value `%s`: %w", containerName, err)
		}

		return nil
	}

	return optionWrap
}

// WithContainerNameFromTest - use custom container name (from test name) instead of random (generated by docker).
// All invalid characters in t.Name() will be repaced to "-"
//
// Example usage:
//
//	WithContainerNameFromTest(t) // "Test/with/invalid/chars" -> "Test-with-invalid-chars"
func WithContainerNameFromTest(t testing.TB) TestContainerOption { // TODO: prefix / postfix are often needed
	var containerName string
	if t != nil {
		containerName = containerNameInvalidCharsRegexp.ReplaceAllString(t.Name(), "-")
	}

	option := func(options *testContainerOptions) (err error) {
		if containerName == "" {
			return fmt.Errorf("%w: t.Name() must not be empty", ErrOptionInvalid)
		}

		options.containerName = containerName

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithContainerNameFromTest with value `%s`: %w", containerName, err)
		}

		return nil
	}

	return optionWrap
}

// WithENV - pass env to container.
//
// Example usage:
//
//	WithENV("USER=root", "PASSWORD=password")
func WithENV(env ...string) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if len(env) == 0 {
			return fmt.Errorf("%w: env must not be empty", ErrOptionInvalid)
		}
		for i, v := range env {
			if v == "" {
				return fmt.Errorf("%w: env[`%d`] must not be empty", ErrOptionInvalid, i)
			}
			if !strings.Contains(v, "=") {
				return fmt.Errorf("%w: env[`%d`] has invalid format", ErrOptionInvalid, i)
			}
		}

		options.env = env

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithENV with value `%#+v`: %w", env, err)
		}

		return nil
	}

	return optionWrap
}

// WithCMD - pass cmd to container.
//
// Example usage:
//
//	WithCMD("server", "start")
func WithCMD(cmd ...string) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if len(cmd) == 0 {
			return fmt.Errorf("%w: cmd must not be empty", ErrOptionInvalid)
		}

		options.cmd = cmd

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithCMD with value `%#+v`: %w", cmd, err)
		}

		return nil
	}

	return optionWrap
}

// WithRetry - customize retry operaton that checks that container started successfully.
// New / NewWithPool func will wait until retry operation successfully winished or retryTimeout
//
// Example usage:
//
//	tcontainer.WithRetry(
//		func(container *dockertest.Resource, apiEndpoints map[int]tcontainer.APIEndpoint) (err error) {
//			return connectToDB(apiEndpoints[5432].IP, apiEndpoints[5432].Port, "user", "pass")
//		},
//		0, // use default retry timeout
//	)
func WithRetry(retryOperation RetryOperation, retryTimeout time.Duration) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if retryOperation == nil {
			return fmt.Errorf("%w: retryOperation must not be nil", ErrOptionInvalid)
		}
		if retryTimeout < 0 {
			return fmt.Errorf("%w: retryTimeout must not be < 0", ErrOptionInvalid)
		}

		options.retryOperation = retryOperation
		if retryTimeout != 0 {
			options.retryTimeout = retryTimeout
		}

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithRetry: %w", err)
		}

		return nil
	}

	return optionWrap
}

// WithExposedPorts - specifies ports to be exposed from the container.
func WithExposedPorts(exposedPorts ...int) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if len(exposedPorts) == 0 {
			return fmt.Errorf("%w: exposedPorts must not be empty", ErrOptionInvalid)
		}

		options.exposedPorts = make([]string, 0, len(exposedPorts))

		for _, exposedPort := range exposedPorts {
			if exposedPort <= 0 {
				return fmt.Errorf("%w: exposedPorts must not be <= 0", ErrOptionInvalid)
			}

			options.exposedPorts = append(options.exposedPorts, strconv.Itoa(exposedPort)+"/tcp")
		}

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithExposedPorts with value `%#+v`: %w", exposedPorts, err)
		}

		return nil
	}

	return optionWrap
}

// WithPortBindings - use custom public ports instead of random.
//
// Input is map[privatePort]publicPort, where:
//   - privatePort is the container port
//   - publicPort is the host machine port
//
// Example usage:
//
//	WithPortBindings(map[int]int{80: 8080, 443: 8443})
func WithPortBindings(portBindings map[privatePort]publicPort) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if len(portBindings) == 0 {
			return fmt.Errorf("%w: portBindings must not be empty", ErrOptionInvalid)
		}

		options.portBindings = make(map[docker.Port][]docker.PortBinding, len(portBindings))

		// TODO: impossible to map one private port to two public because of privatePort is key
		for privatePort, publicPort := range portBindings {
			dockerPort := docker.Port(fmt.Sprintf("%d/tcp", privatePort))
			portBinding := docker.PortBinding{
				HostIP:   "0.0.0.0",
				HostPort: strconv.Itoa(publicPort),
			}

			options.portBindings[dockerPort] = append(options.portBindings[dockerPort], portBinding)
		}

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithPortBindings with value `%+v`: %w", portBindings, err)
		}

		return nil
	}

	return optionWrap
}

// WithExpiry - stops container after `containerExpiry` time.
//
// Note: Have default value, but you can rewrite it by set containerExpiry to 0
// if you don't want container to expire.
func WithExpiry(containerExpiry time.Duration) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		options.containerExpiry = containerExpiry

		return nil
	}

	return option
}

// WithAutoremove - removes container after it stops (even by expiry).
func WithAutoremove(autoremoveContainer bool) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		options.autoremoveContainer = autoremoveContainer

		return nil
	}

	return option
}

// WithReuseContainer - reuse container if it exists instead of error.
//
// Note:
//   - Must not be used with WithRemoveContainerOnExists (ErrOptionsConflict).
//   - You can get error if existed container have other TestContainerOptions (other port mapping or image for example).
//   - You can set `reuseTimeout` in order to change default timeout (wait until unpause / start existed container).
//   - You can set `recreateOnError` in order to recreate container instead of error
//     when existed container have other TestContainerOptions
//     or failed to start container for reuse (if it was stopped/paused).
func WithReuseContainer(reuseContainer bool, reuseTimeout time.Duration, recreateOnError bool) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if !reuseContainer {
			return nil
		}

		if options.removeContainerOnExists {
			return fmt.Errorf("%w: WithRemoveContainerOnExists is `true`", ErrOptionConflict)
		}
		if reuseTimeout != 0 {
			if reuseTimeout < time.Second {
				// because of backoff initial timeout is second
				return fmt.Errorf("%w: reuseTimeout must not be < time.Second", ErrOptionInvalid)
			}

			options.reuseContainerTimeout = reuseTimeout
		}

		options.reuseContainer = reuseContainer
		options.reuseContainerRecreateOnErr = recreateOnError

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithReuseContainer with value `%v, %v`: %w", reuseContainer, recreateOnError, err)
		}

		return nil
	}

	return optionWrap
}

// WithRemoveContainerOnExists - remove container if it exists instead of error.
//
// Note: Must not be used with WithReuseContainer (ErrOptionsConflict).
func WithRemoveContainerOnExists(removeContainerOnExists bool) TestContainerOption {
	option := func(options *testContainerOptions) (err error) {
		if !removeContainerOnExists {
			return nil
		}

		if options.reuseContainer {
			return fmt.Errorf("%w: WithReuseContainer is `true`", ErrOptionConflict)
		}

		options.removeContainerOnExists = removeContainerOnExists

		return nil
	}

	optionWrap := func(options *testContainerOptions) (err error) {
		err = option(options)
		if err != nil {
			return fmt.Errorf("failed to apply WithRemoveContainerOnExists with value `%v`: %w", removeContainerOnExists, err)
		}

		return nil
	}

	return optionWrap
}
