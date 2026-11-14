package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInitBytesStatsETA(t *testing.T) {
	r := require.New(t)
	_, ok := InitBytesStats{Total: 100, Done: 10, BytesPerSecond: 5}.ETA()
	r.False(ok, "listing still running")
	_, ok = InitBytesStats{Total: 100, Done: 10, ListingComplete: true}.ETA()
	r.False(ok, "no throughput")
	eta, ok := InitBytesStats{Total: 100, Done: 10, BytesPerSecond: 5, ListingComplete: true}.ETA()
	r.True(ok)
	r.Equal(18*time.Second, eta)
	eta, ok = InitBytesStats{Total: 100, Done: 100, BytesPerSecond: 5, ListingComplete: true}.ETA()
	r.True(ok)
	r.Zero(eta)
}
