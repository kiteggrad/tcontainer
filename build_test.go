package tcontainer

import (
	"context"
	"fmt"
	"testing"

	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

// buildTestImage - creates minimal configureated image for tests.
func buildTestImage(pool Pool, customOpts ...BuildOption) (image *docker.Image, err error) {
	opts := append([]BuildOption{
		func(options *BuildOptions) (err error) {
			options.Dockerfile = "internal/testing/Dockerfile.test"
			options.ContextDir = "."
			return nil
		},
	}, customOpts...)

	image, err = pool.BuildAndGet(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to BuildAndGet: %w", err)
	}

	return image, nil
}

func Test_Build(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	pool := MustNewPool("")

	image, err := buildTestImage(pool)
	require.NoError(err)
	t.Cleanup(func() { require.NoError(pool.Pool.Client.RemoveImage(image.ID)) })

	require.NotEmpty(image)
}

func Test_BuildAndGet(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	pool := MustNewPool("")

	image, err := buildTestImage(pool)
	require.NoError(err)
	t.Cleanup(func() { require.NoError(pool.Pool.Client.RemoveImage(image.ID)) })

	require.NotEmpty(image)
}

func Test_BuildAndGet_AlreadyExists(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	pool := MustNewPool("")

	// create first image
	image1, err := buildTestImage(pool, func(options *BuildOptions) (err error) {
		options.ImageName = "test_buildandget_alreadyexists"
		return nil
	})
	require.NoError(err)
	t.Cleanup(func() { require.NoError(pool.Pool.Client.RemoveImage(image1.ID)) })

	require.NotEmpty(image1)

	// create second image
	image2, err := buildTestImage(pool, func(options *BuildOptions) (err error) {
		options.ImageName = "test_buildandget_alreadyexists"
		return nil
	})
	require.NoError(err)
	t.Cleanup(func() { require.NoError(pool.Pool.Client.RemoveImage(image2.ID)) })

	require.NotEmpty(image2)

	// check first image still exists
	image1Data, err := pool.findImageByUUID(context.Background(), image1.Config.Labels[ImageLabelUUID])
	require.NoError(err)
	require.NotEmpty(image1Data)
}

func Test_BuildOptions_WithImageName(t *testing.T) { //nolint:dupl // similar to WithContainerName but different
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
			args: args{nameParts: []string{"FirstSecond"}},
			want: want{name: "first_second"},
		},
		{
			name: "join_parts",
			args: args{nameParts: []string{"1", "2", "3"}},
			want: want{name: "1/2/3"},
		},
		{
			name: "remove_empty_parts",
			args: args{nameParts: []string{"1", "", "3"}},
			want: want{name: "1/3"},
		},
		{
			name: "special_chars",
			args: args{nameParts: []string{"1|2/3_4-5:6"}},
			want: want{name: "1/2/3_4/5:6"},
		},
		{
			name: "special_chars/duplicated",
			args: args{nameParts: []string{"1||2=3+4|", "?/5"}},
			want: want{name: "1/2/3/4/5"},
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require := require.New(t)

			// run logic
			opts := BuildOptions{}
			err := WithImageName(test.args.nameParts...)(&opts)
			require.NoError(err)
			require.Equal(test.want.name, opts.ImageName)
		})
	}
}
