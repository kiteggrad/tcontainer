package tcontainer

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/ory/dockertest/v3/docker"
)

// Build a new image.
//   - Rewrites old image with new one if they have the same name.
//   - Old image with the same name won't be removed, but it will lose it's name.
func (p Pool) Build(ctx context.Context, buildOptions ...BuildOption) (err error) {
	options, err := ApplyBuildOptions(uuid.NewString(), buildOptions...)
	if err != nil {
		return fmt.Errorf("failed to applyBuildOptions: %w", err)
	}

	return p.buildImage(ctx, options)
}

// BuildAndGet a new image.
//   - Rewrites old image with new one if they have the same name.
//   - Old image with the same name won't be removed, but it will lose it's name.
//   - Returns information about the created image.
func (p Pool) BuildAndGet(ctx context.Context, buildOptions ...BuildOption) (image *docker.Image, err error) {
	options, err := ApplyBuildOptions(uuid.NewString(), buildOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to applyBuildOptions: %w", err)
	}

	err = p.buildImage(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to buildImage: %w", err)
	}

	imageUUID, ok := options.Labels[ImageLabelUUID]
	if !ok {
		return nil, errors.New("not found imageUUID in options.Labels")
	}

	image, err = p.inspectImageByUUID(ctx, imageUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspectImageByUUID: %w", err)
	}

	return image, nil
}

func (p Pool) buildImage(ctx context.Context, options BuildOptions) (err error) {
	return p.Pool.Client.BuildImage(options.toDockertest(ctx)) //nolint:wrapcheck
}
