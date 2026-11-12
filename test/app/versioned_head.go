package app

import "net/http"

// gofakes3 v1.2.0 routes HEAD ?versionId to headObject but headObject calls
// storage.HeadObject without using versionID. Its GET route does select the
// requested version. Keep the embedded fixture faithful to S3 by using that
// route for versioned HEAD and discarding the body without buffering it.
func versionAwareFakeServer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && r.URL.Query().Get("versionId") != "" {
			get := r.Clone(r.Context())
			get.Method = http.MethodGet
			next.ServeHTTP(discardHeadBody{w}, get)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type discardHeadBody struct{ http.ResponseWriter }

func (discardHeadBody) Write(p []byte) (int, error) { return len(p), nil }
