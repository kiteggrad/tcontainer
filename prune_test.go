package tcontainer

import (
	"context"
	"testing"

	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_pruneContainers(t *testing.T) { //nolint:paralleltest
	require := require.New(t)
	assert := assert.New(t)

	// create containers using this package
	containerName := t.Name()
	pool, container, err := runBusybox(context.Background(), WithContainerName(containerName+"1"))
	require.NoError(err)
	_, container2, err := runBusybox(context.Background(), WithContainerName(containerName+"2"))
	require.NoError(err)

	// create some other container
	_, container3, err := runBusybox(context.Background(), func(options *RunOptions) (err error) {
		options.Labels = nil
		return nil
	})
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(container3.Close()) })

	// do prune
	err = pool.pruneContainers(context.Background())
	require.NoError(err)

	// check no side containers was removed
	notRemovedContainers, err := pool.Pool.Client.ListContainers(docker.ListContainersOptions{
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

func Test_pruneImages(t *testing.T) { //nolint:paralleltest
	require := require.New(t)
	assert := assert.New(t)

	pool := MustNewPool("")

	// create images using this package
	image, err := buildTestImage(pool)
	require.NoError(err)
	image2, err := buildTestImage(pool)
	require.NoError(err)

	// create some side image
	sideImage, err := buildTestImage(pool, func(options *BuildOptions) (err error) {
		options.ImageName = "tcontainer/side_image:latest"
		delete(options.Labels, DefaultLabelKeyValue)
		return nil
	})
	require.NoError(err)
	t.Cleanup(func() { assert.NoError(pool.Pool.Client.RemoveImage(sideImage.ID)) })

	// prune
	err = pool.pruneImages(context.Background())
	require.NoError(err)

	// check images was deleted
	_, err = pool.Pool.Client.InspectImage(image.ID)
	require.ErrorIs(err, docker.ErrNoSuchImage)
	_, err = pool.Pool.Client.InspectImage(image2.ID)
	require.ErrorIs(err, docker.ErrNoSuchImage)

	// check side image wasn't deleted
	_, err = pool.Pool.Client.InspectImage(sideImage.ID)
	require.NoError(err)
}
