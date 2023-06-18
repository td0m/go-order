package main

import (
	"embed"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testdata embed.FS

func TestSortAST(t *testing.T) {
	dirs, err := testdata.ReadDir("testdata")
	require.NoError(t, err)

	paths := make([]string, len(dirs))
	for i, entry := range dirs {
		require.True(t, entry.IsDir())
		paths[i] = path.Join("testdata", entry.Name())
	}

	config := Config{
		SortAlphabetically: true,
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			in, err := os.ReadFile(path.Join(p, "in.txt"))
			require.NoError(t, err)

			expected, err := os.ReadFile(path.Join(p, "expected.txt"))
			require.NoError(t, err)

			actual, err := sortFile(in, config)
			require.NoError(t, err)

			require.Equal(t, string(expected), string(actual.Bytes()))
		})
	}
}
