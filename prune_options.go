package tcontainer

import (
	"fmt"
)

type (
	// PruneOptions for (Pool).Prune function.
	PruneOptions struct {
		PruneContainersOption PruneContainersOption
		PruneImagesOption     PruneImagesOption
	}

	// PruneContainersOption for (Pool).Prune function.
	PruneContainersOption struct {
		Filters map[string][]string
	}

	// PruneImagesOption for (Pool).Prune function.
	PruneImagesOption struct {
		Filters map[string][]string
	}

	// PruneOption - option for (Pool).Prune function.
	// See [ApplyPruneOptions].
	PruneOption func(options *PruneOptions) (err error)
)

// ApplyPruneOptions sets defaults and apply custom options.
// Options aplies in order they passed.
//
// Each option rewrites previous value.
func ApplyPruneOptions(customOpts ...PruneOption) (
	options PruneOptions, err error,
) {
	options = options.getDefault()

	for _, customOpt := range customOpts {
		err = customOpt(&options)
		if err != nil {
			return PruneOptions{}, err
		}
	}

	err = options.validate()
	if err != nil {
		return PruneOptions{}, fmt.Errorf("failed to options.validate: %w", err)
	}

	return options, nil
}

func (o PruneOptions) getDefault() (defaultPruneOptions PruneOptions) {
	return PruneOptions{
		PruneContainersOption: PruneContainersOption{
			Filters: map[string][]string{"label": {DefaultLabelKeyValue + "=" + DefaultLabelKeyValue}},
		},
		PruneImagesOption: PruneImagesOption{
			Filters: map[string][]string{"label": {DefaultLabelKeyValue + "=" + DefaultLabelKeyValue}},
		},
	}
}

func (o PruneOptions) validate() (err error) {
	return nil
}
