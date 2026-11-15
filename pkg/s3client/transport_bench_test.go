package s3client

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// Micro-benchmark of transport knobs against a local TLS server (loopback, server in the same process, so the
// CPU numbers include the server side; compare the variants with each other, not with a WAN).
//
//	GOMAXPROCS=2 go test ./pkg/s3client -run '^$' -bench Transport -benchtime 3x

func cpuSecs() float64 {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Utime.Sec+ru.Stime.Sec) + float64(ru.Utime.Usec+ru.Stime.Usec)/1e6
}

func BenchmarkTransportTLS(b *testing.B) {
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = byte(i * 7)
	}
	const total = 512 << 20
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			_, _ = io.Copy(io.Discard, r.Body)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(total))
		for n := 0; n < total; n += len(chunk) {
			_, _ = w.Write(chunk)
		}
	}))
	srv.EnableHTTP2 = false
	srv.StartTLS()
	defer srv.Close()

	variants := map[string]func() *http.Transport{
		"default": func() *http.Transport { return &http.Transport{DisableCompression: true} },
		"buf64k": func() *http.Transport {
			return &http.Transport{DisableCompression: true, ReadBufferSize: 64 << 10, WriteBufferSize: 64 << 10}
		},
	}
	for _, dir := range []string{"download", "upload"} {
		for name, mk := range variants {
			b.Run(dir+"/"+name, func(b *testing.B) {
				tr := mk()
				tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
				defer tr.CloseIdleConnections()
				c := &http.Client{Transport: tr}
				b.SetBytes(total)
				cpu0 := cpuSecs()
				t0 := time.Now()
				for i := 0; i < b.N; i++ {
					if dir == "download" {
						resp, err := c.Get(srv.URL)
						if err != nil {
							b.Fatal(err)
						}
						_, _ = io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					} else {
						req, _ := http.NewRequest(http.MethodPut, srv.URL, &repeatReader{chunk: chunk, left: total})
						req.ContentLength = total
						resp, err := c.Do(req)
						if err != nil {
							b.Fatal(err)
						}
						resp.Body.Close()
					}
				}
				gib := float64(total) * float64(b.N) / (1 << 30)
				b.ReportMetric(float64(total)*float64(b.N)/(1<<20)/time.Since(t0).Seconds(), "MiB/s")
				b.ReportMetric((cpuSecs()-cpu0)/gib, "cpu-s/GiB(client+server)")
			})
		}
	}
}

type repeatReader struct {
	chunk []byte
	left  int
}

func (r *repeatReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	n := min(len(p), len(r.chunk), r.left)
	copy(p, r.chunk[:n])
	r.left -= n
	return n, nil
}
