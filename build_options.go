package tcontainer

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/huandu/xstrings"
	"github.com/ory/dockertest/v3/docker"
)

const (
	ImageLabelUUID = DefaultLabelKeyValue + ".uuid"
)

var imageNameInvalidCharsRegexp = regexp.MustCompile("[^a-zA-Z0-9_.-:]")

type (
	// BuildOptions for (Pool).Build / (Pool).BuildAndGet functions.
	BuildOptions struct {
		ImageName           string
		Dockerfile          string
		ContextDir          string
		BuildArgs           []docker.BuildArg
		Platform            string
		NoCache             bool
		CacheFrom           []string
		SuppressOutput      bool
		Pull                bool
		RmTmpContainer      bool
		ForceRmTmpContainer bool
		RawJSONStream       bool
		Memory              int64
		Memswap             int64
		ShmSize             int64
		CPUShares           int64
		CPUQuota            int64
		CPUPeriod           int64
		CPUSetCPUs          string
		Labels              map[string]string
		InputStream         io.Reader
		OutputStream        io.Writer
		ErrorStream         io.Writer
		Remote              string
		Auth                docker.AuthConfiguration
		AuthConfigs         docker.AuthConfigurations
		Ulimits             []docker.ULimit
		NetworkMode         string
		InactivityTimeout   time.Duration
		CgroupParent        string
		SecurityOpt         []string
		Target              string
		Version             string
		Outputs             string
		ExtraHosts          string
	}

	// BuildOption - option for (Pool).Build / (Pool).BuildAndGet functions.
	// See [ApplyBuildOptions].
	BuildOption func(options *BuildOptions) (err error)
)

// WithImageName - use custom image name instead of random (generated by docker).
//   - All invalid characters will be repaced to "/".
//   - Not empty nameParts will be joined with "/" separator, empty parts will be removed.
//   - Snake case will be applied.
//
// Example:
//
//	WithImageName(t.Name(), "redis") // "Test/withInvalid|chars", "redis" -> "Test/with_invalid/chars/redis"
func WithImageName(nameParts ...string) BuildOption {
	return func(options *BuildOptions) (err error) {
		const separator = "/"

		// remove empty parts
		nameParts = slices.DeleteFunc(nameParts, func(s string) bool { return s == "" })

		// join parts
		name := strings.Join(nameParts, separator)

		// replace invalid chars
		name = imageNameInvalidCharsRegexp.ReplaceAllString(name, separator)
		name = xstrings.ToSnakeCase(name)

		// replace delimiter duplications
		for strings.Contains(name, separator+separator) {
			name = strings.ReplaceAll(name, separator+separator, separator)
		}

		// set option
		options.ImageName = name

		return nil
	}
}

// ApplyBuildOptions sets defaults and apply custom options.
// Options aplies in order they passed.
//
// Each option rewrites previous value
//
//	ApplyBuildOptions(WithImageName("first"), WithImageName("second")) // "second"
func ApplyBuildOptions(uuid string, customOpts ...BuildOption) (
	options BuildOptions, err error,
) {
	options = options.getDefault(uuid)

	for _, customOpt := range customOpts {
		err = customOpt(&options)
		if err != nil {
			return BuildOptions{}, err
		}
	}

	err = options.validate()
	if err != nil {
		return BuildOptions{}, fmt.Errorf("failed to options.validate: %w", err)
	}

	return options, nil
}

func (o BuildOptions) getDefault(uuid string) (defaultBuildOptions BuildOptions) {
	return BuildOptions{ //nolint:exhaustruct
		ImageName:    "",
		Dockerfile:   "",
		ContextDir:   "",
		BuildArgs:    []docker.BuildArg{},
		Platform:     "",
		OutputStream: io.Discard,
		Labels: map[string]string{
			DefaultLabelKeyValue: DefaultLabelKeyValue,
			ImageLabelUUID:       uuid,
		},
	}
}

func (o BuildOptions) validate() (err error) {
	return nil
}

func (o BuildOptions) toDockertest(ctx context.Context) (dockertestBuildOptions docker.BuildImageOptions) {
	return docker.BuildImageOptions{
		Name:                o.ImageName,
		Dockerfile:          o.Dockerfile,
		NoCache:             o.NoCache,
		CacheFrom:           o.CacheFrom,
		SuppressOutput:      o.SuppressOutput,
		Pull:                o.Pull,
		RmTmpContainer:      o.RmTmpContainer,
		ForceRmTmpContainer: o.ForceRmTmpContainer,
		RawJSONStream:       o.RawJSONStream,
		Memory:              o.Memory,
		Memswap:             o.Memswap,
		ShmSize:             o.ShmSize,
		CPUShares:           o.CPUShares,
		CPUQuota:            o.CPUQuota,
		CPUPeriod:           o.CPUPeriod,
		CPUSetCPUs:          o.CPUSetCPUs,
		Labels:              o.Labels,
		InputStream:         o.InputStream,
		OutputStream:        o.OutputStream,
		ErrorStream:         o.ErrorStream,
		Remote:              o.Remote,
		Auth:                o.Auth,
		AuthConfigs:         o.AuthConfigs,
		ContextDir:          o.ContextDir,
		Ulimits:             o.Ulimits,
		BuildArgs:           o.BuildArgs,
		NetworkMode:         o.NetworkMode,
		InactivityTimeout:   o.InactivityTimeout,
		CgroupParent:        o.CgroupParent,
		SecurityOpt:         o.SecurityOpt,
		Target:              o.Target,
		Version:             o.Version,
		Platform:            o.Platform,
		Outputs:             o.Outputs,
		ExtraHosts:          o.ExtraHosts,
		Context:             ctx,
	}
}
