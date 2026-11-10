package s3client

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/s3"
)

func TestStorageTransport(t *testing.T) {
	r := require.New(t)
	a := s3.StorageAddress{Address: "http://tr-a:9000"}
	t1, t2 := storageTransport(a), storageTransport(a)
	r.Same(t1, t2, "transport is shared per storage")
	r.Equal(1024, t1.MaxIdleConns)
	r.Equal(512, t1.MaxIdleConnsPerHost)
	r.Equal(0, t1.MaxConnsPerHost)
	r.True(t1.DisableCompression)
	r.False(t1.ForceAttemptHTTP2)
	r.NotNil(t1.Proxy)

	// other storage: other transport
	r.NotSame(t1, storageTransport(s3.StorageAddress{Address: "http://tr-b:9000"}))

	// configured limits and changed settings of the same address get a new transport
	a.MaxIdleConns, a.MaxIdleConnsPerHost = 10, 5
	t3 := storageTransport(a)
	r.NotSame(t1, t3)
	r.Equal(10, t3.MaxIdleConns)
	r.Equal(5, t3.MaxIdleConnsPerHost)
	r.Same(t3, storageTransport(a))
}
