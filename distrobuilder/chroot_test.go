package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/shared"
)

func TestManagePackages(t *testing.T) {
	sets := []shared.DefinitionPackagesSet{
		{
			Packages: []string{"foo"},
			Action:   "install",
		},
		{
			Packages: []string{"bar"},
			Action:   "install",
		},
		{
			Packages: []string{"baz"},
			Action:   "remove",
		},
		{
			Packages: []string{"lorem"},
			Action:   "remove",
		},
		{
			Packages: []string{"ipsum"},
			Action:   "install",
		},
		{
			Packages: []string{"dolor"},
			Action:   "remove",
		},
	}

	optimizedSets := optimizePackageSets(sets)
	require.Len(t, optimizedSets, 4)
	require.Equal(t, optimizedSets[0], shared.DefinitionPackagesSet{Action: "install", Packages: []string{"foo", "bar"}})
	require.Equal(t, optimizedSets[1], shared.DefinitionPackagesSet{Action: "remove", Packages: []string{"baz", "lorem"}})
	require.Equal(t, optimizedSets[2], shared.DefinitionPackagesSet{Action: "install", Packages: []string{"ipsum"}})
	require.Equal(t, optimizedSets[3], shared.DefinitionPackagesSet{Action: "remove", Packages: []string{"dolor"}})

	sets = []shared.DefinitionPackagesSet{
		{
			Packages: []string{"foo"},
			Action:   "install",
		},
	}

	optimizedSets = optimizePackageSets(sets)
	require.Len(t, optimizedSets, 1)
	require.Equal(t, optimizedSets[0], shared.DefinitionPackagesSet{Action: "install", Packages: []string{"foo"}})

	sets = []shared.DefinitionPackagesSet{}
	optimizedSets = optimizePackageSets(sets)
	require.Len(t, optimizedSets, 0)
}
