package tcontainer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ory/dockertest/v3/docker"
)

// Prune - remove containers and images created by this package.
func (p Pool) Prune(ctx context.Context, customOptions ...PruneOption) (err error) {
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	wg.Add(2) //nolint:mnd
	go func() {
		defer wg.Done()
		removeErr := p.pruneContainers(ctx, customOptions...)
		if removeErr != nil {
			mu.Lock()
			err = errors.Join(err, fmt.Errorf("failed to pruneContainers: %w", removeErr))
			mu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		removeErr := p.pruneImages(ctx)
		if removeErr != nil {
			mu.Lock()
			err = errors.Join(err, fmt.Errorf("failed to pruneImages: %w", removeErr))
			mu.Unlock()
		}
	}()
	wg.Wait()

	return nil
}

func (p Pool) pruneContainers(ctx context.Context, customOptions ...PruneOption) (err error) {
	options, err := ApplyPruneOptions(customOptions...)
	if err != nil {
		return fmt.Errorf("failed to applyPruneOptions: %w", err)
	}

	containers, err := p.Pool.Client.ListContainers(docker.ListContainersOptions{
		All:     true,
		Size:    false,
		Limit:   0,
		Since:   "",
		Before:  "",
		Filters: options.PruneContainersOption.Filters,
		Context: ctx,
	})
	if err != nil {
		return fmt.Errorf("failed to ListContainers: %w", err)
	}

	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	for _, container := range containers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			removeErr := p.Pool.Client.RemoveContainer(docker.RemoveContainerOptions{
				ID:            container.ID,
				RemoveVolumes: true,
				Force:         true,
				Context:       ctx,
			})
			if removeErr != nil {
				mu.Lock()
				err = errors.Join(err, fmt.Errorf("failed to RemoveContainer `%s`: %w", container.ID, removeErr))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return nil
}

func (p Pool) pruneImages(ctx context.Context, customOptions ...PruneOption) (err error) {
	options, err := ApplyPruneOptions(customOptions...)
	if err != nil {
		return fmt.Errorf("failed to applyPruneOptions: %w", err)
	}

	images, err := p.Pool.Client.ListImages(docker.ListImagesOptions{
		Filters: options.PruneImagesOption.Filters,
		Filter:  "",
		All:     true,
		Digests: false,
		Context: ctx,
	})
	if err != nil {
		return fmt.Errorf("failed to ListImages: %w", err)
	}

	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	for _, image := range images {
		wg.Add(1)
		go func() {
			defer wg.Done()
			removeErr := p.Pool.Client.RemoveImageExtended(image.ID, docker.RemoveImageOptions{
				Force:   true,
				NoPrune: false,
				Context: ctx,
			})
			if removeErr != nil {
				mu.Lock()
				err = errors.Join(err, fmt.Errorf("failed to RemoveImageExtended `%s`: %w", image.ID, removeErr))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	return nil
}
