package copy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProgressReaderCapsAtSize(t *testing.T) {
	var got []int64
	p := &progressReader{fn: func(n int64) { got = append(got, n) }, size: 100}
	for _, n := range []int{40, 40, 40, 0} {
		r, err := p.Read(make([]byte, n))
		require.NoError(t, err)
		require.Equal(t, n, r)
	}
	// the third read (a re-sent part) is capped to the object size, empty reads report nothing
	require.Equal(t, []int64{40, 80, 100}, got)
}
