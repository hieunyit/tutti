package auth

import (
	"net/http"
	"time"

	mclient "github.com/minio/minio-go/v7"
)

const maxHeaderClockSkew = 15 * time.Minute

func checkHeaderTime(t, now time.Time) error {
	if t.Before(now.Add(-maxHeaderClockSkew)) || t.After(now.Add(maxHeaderClockSkew)) {
		return mclient.ErrorResponse{
			Code:       "RequestTimeTooSkewed",
			Message:    "The difference between the request time and the server's time is too large.",
			StatusCode: http.StatusForbidden,
		}
	}
	return nil
}
