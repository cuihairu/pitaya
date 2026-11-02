package compression

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/topfreegames/pitaya/v3/pkg/helpers"
)

var ins = []struct {
	name string
	data string
}{
	{"compression_deflate_test_1", "test"},
	{"compression_deflate_test_2", "{a:1,b:2}"},
	{"compression_deflate_test_3", "Neque porro quisquam est qui dolorem ipsum quia dolor sit amet, consectetur, adipisci velit"},
}

func TestCompressionDeflate(t *testing.T) {
	for _, in := range ins {
		t.Run(in.name, func(t *testing.T) {
			b, err := DeflateData([]byte(in.data))
			require.NoError(t, err)

			// zlib output size and byte layout vary by Go version, so assert
			// the round-trip property (deflated bytes inflate back to the
			// original) instead of exact golden bytes.
			result, err := InflateData(b)
			require.NoError(t, err)
			assert.Equal(t, in.data, string(result))
		})
	}
}

func TestCompressionInflate(t *testing.T) {
	for _, in := range ins {
		t.Run(in.name, func(t *testing.T) {
			inputFile := filepath.Join("fixtures", in.name+".golden")
			input := helpers.ReadFile(t, inputFile)

			result, err := InflateData(input)
			require.NoError(t, err)

			assert.Equal(t, string(result), in.data)
		})
	}
}

func TestCompressionInflateIncorrectData(t *testing.T) {
	t.Run("compression_deflate_incorrect_data", func(t *testing.T) {
		input := "arbitrary data"

		result, err := InflateData([]byte(input))
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestCompressionInflateZipBombRejected(t *testing.T) {
	// A highly compressible payload larger than MaxInflatedSize must be
	// rejected instead of inflated into an unbounded allocation.
	big := make([]byte, MaxInflatedSize+1)
	b, err := DeflateData(big)
	require.NoError(t, err)
	require.Less(t, len(b), len(big)/100, "test sanity: big zero buffer should compress small")

	result, err := InflateData(b)
	require.Error(t, err)
	assert.Nil(t, result)
}
