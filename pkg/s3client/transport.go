package s3client

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hieunyit/tutti/pkg/s3"
)

const (
	defaultMaxIdleConns        = 1024
	defaultMaxIdleConnsPerHost = 512
)

type transportKey struct {
	address        string
	secure         bool
	maxIdle        int
	maxIdlePerHost int
}

var (
	transportsMu sync.Mutex
	transports   = map[transportKey]*http.Transport{}
)

// storageTransport returns the HTTP transport shared by all clients (of all users, S3/AWS/minio)
// of one storage. A changed address, TLS mode or connection limits yields a new transport.
func storageTransport(conf s3.StorageAddress) *http.Transport {
	maxIdle, perHost := conf.MaxIdleConns, conf.MaxIdleConnsPerHost
	if maxIdle <= 0 {
		maxIdle = defaultMaxIdleConns
	}
	if perHost <= 0 {
		perHost = defaultMaxIdleConnsPerHost
	}
	key := transportKey{address: conf.Address, secure: conf.IsSecure, maxIdle: maxIdle, maxIdlePerHost: perHost}
	transportsMu.Lock()
	defer transportsMu.Unlock()
	if t, ok := transports[key]; ok {
		return t
	}
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          maxIdle,
		MaxIdleConnsPerHost:   perHost,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// the proxy passes bytes through untouched
		DisableCompression: true,
	}
	// drop transports of the same address that were replaced by a config change
	for k, old := range transports {
		if k.address == conf.Address {
			old.CloseIdleConnections()
			delete(transports, k)
		}
	}
	transports[key] = t
	return t
}
