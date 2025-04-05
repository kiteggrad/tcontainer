package tcontainer

import (
	"context"
	"errors"
	"fmt"

	"github.com/cenkalti/backoff/v5"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

// Run - creates and runs new test container.
func (p Pool) Run(
	ctx context.Context, repository string, customOpts ...RunOption,
) (container *dockertest.Resource, err error) {
	options, err := ApplyRunOptions(repository, customOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to applyTestContainerOptions: %w", err)
	}

	return p.run(ctx, options)
}

func (p Pool) run(
	ctx context.Context, options RunOptions,
) (container *dockertest.Resource, err error) {
	container, err = p.initContainer(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to initContainer: %w", err)
	}

	if options.ContainerExpiry != 0 {
		err = container.Expire(uint(options.ContainerExpiry.Seconds()))
		if err != nil {
			_ = p.Pool.Purge(container)
			return nil, fmt.Errorf("failed to container.Expire: %w", err)
		}
	}

	if options.Retry.Operation != nil {
		_, err = backoff.Retry(
			ctx,
			func() (_ struct{}, err error) { return struct{}{}, options.Retry.Operation(ctx, container) },
			backoff.WithBackOff(options.Retry.Backoff),
		)
		if err != nil {
			_ = p.Pool.Purge(container)
			return nil, fmt.Errorf("failed to retry: %w", err)
		}
	}

	return container, nil
}

func (p Pool) initContainer(
	ctx context.Context, options RunOptions,
) (container *dockertest.Resource, err error) {
	container, err = p.createAndStartContainer(options)
	switch {
	case err == nil:
		return container, nil

	case errors.Is(err, ErrContainerAlreadyExists) && options.Reuse.Reuse:
		container, err = p.reuseOrRecreateContainer(ctx, options)
		if err != nil {
			return nil, fmt.Errorf("failed to reuseOrRecreateContainer: %w", err)
		}

		return container, nil

	case errors.Is(err, ErrContainerAlreadyExists) && options.RemoveOnExists:
		container, err := p.recreateContainer(options)
		if err != nil {
			return nil, fmt.Errorf("failed to recreateContainer by options.RemoveOnExists: %w", err)
		}

		return container, nil

	default:
		return nil, fmt.Errorf("failed to createAndStartContainer: %w", err)
	}
}

func (p Pool) createAndStartContainer(
	options RunOptions,
) (container *dockertest.Resource, err error) {
	container, err = p.Pool.RunWithOptions(
		options.toDockertest(),
		func(hc *docker.HostConfig) { *hc = options.HostConfig },
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dockerPool.RunWithOptions: %w", err)
	}

	return container, nil
}

// reuseOrRecreateContainer - try to reuse container, or recreate (optional) if failed to reuse.
func (p Pool) reuseOrRecreateContainer(
	ctx context.Context, options RunOptions,
) (container *dockertest.Resource, err error) {
	container, err = p.reuseContainer(ctx, options)
	switch {
	case err == nil:
		return container, nil

	case options.Reuse.RecreateOnErr:
		err = fmt.Errorf("failed to reuseContainer: %w", err)

		container, recreateErr := p.recreateContainer(options)
		if recreateErr != nil {
			recreateErr = fmt.Errorf("failed to recreateContainer after reuseContainer err: %w", err)
			return nil, errors.Join(err, recreateErr)
		}

		return container, nil

	default:
		return nil, fmt.Errorf("failed to reuseContainer: %w", err)
	}
}

func (p Pool) reuseContainer(
	ctx context.Context, options RunOptions,
) (container *dockertest.Resource, err error) {
	try := func() (container *dockertest.Resource, err error) {
		var ok bool
		container, ok = p.Pool.ContainerByName(fmt.Sprintf("^%s$", options.Name))
		if !ok {
			return nil, backoff.Permanent(fmt.Errorf("failed to p.ContainerByName `%s`: %w", options.Name, err))
		}

		err = checkContainerState(container.Container)
		if err != nil {
			err = fmt.Errorf("failed to checkContainerState: %w", err)
			if errors.Is(err, ErrUnreusableState) {
				return nil, backoff.Permanent(err)
			}
			return container, err
		}

		for _, checkContainerConfig := range options.Reuse.ConfigChecks {
			err = checkContainerConfig(container.Container, options)
			if err != nil {
				return nil, backoff.Permanent(fmt.Errorf("%w: failed to checkContainerConfig: %w", ErrReuseContainerConflict, err))
			}
		}

		return container, nil
	}

	container, err = try()
	if err == nil {
		return container, nil
	} else if errors.As(err, ptr((*backoff.PermanentError)(nil))) {
		return nil, err
	}

	err = repairForReuse(p.Pool.Client, container.Container)
	if err != nil {
		return nil, fmt.Errorf("failed to repairForReuse: %w", err)
	}

	container, err = backoff.Retry(ctx, try, backoff.WithBackOff(options.Reuse.Backoff))
	if err != nil {
		return nil, fmt.Errorf("failed to retry after repairForReuse: %w", err)
	}

	return container, nil
}

// repairForReuse - do something to fix container state, do nothing if container is ok.
func repairForReuse(client *docker.Client, container *docker.Container) (err error) {
	switch {
	case checkContainerState(container) == nil:
		return nil

	case container.State.Restarting:
		return nil

	case container.State.Paused:
		err = client.UnpauseContainer(container.ID)
		if err != nil {
			return fmt.Errorf("failed to UnpauseContainer: %w", err)
		}

	case container.State.Status == "exited":
		err = client.StartContainer(container.ID, container.HostConfig)
		if err != nil {
			return fmt.Errorf("failed to StartContainer on `exited` status: %w", err)
		}

	case container.State.OOMKilled, container.State.Dead, container.State.RemovalInProgress:
		return backoff.Permanent(fmt.Errorf("%w: `%s`", ErrUnreusableState, container.State.String())) //nolint:wrapcheck

	default:
		return fmt.Errorf("unexpected Container.State `%s`", container.State.StateString())
	}

	return nil
}

// checkContainerState - checks that container is ready.
func checkContainerState(container *docker.Container) (err error) {
	switch {
	case container.State.Paused:
		return errors.New("still paused")

	case container.State.Status == "exited":
		return errors.New("still exited")

	case container.State.Restarting:
		return errors.New("still restarting")

	case container.State.Running:
		return nil

	case container.State.OOMKilled, container.State.Dead, container.State.RemovalInProgress:
		return fmt.Errorf("%w: %s", ErrUnreusableState, container.State.String())

	default:
		return fmt.Errorf("unexpected Container.State `%s`", container.State.StateString())
	}
}

func (p Pool) recreateContainer(
	options RunOptions,
) (container *dockertest.Resource, err error) {
	err = p.Pool.RemoveContainerByName(fmt.Sprintf("^%s$", options.Name))
	if err != nil {
		return nil, fmt.Errorf("failed to p.RemoveContainerByName: %w", err)
	}

	container, err = p.createAndStartContainer(options)
	if err != nil {
		return nil, fmt.Errorf("failed to createAndStartContainer: %w", err)
	}

	return container, nil
}
