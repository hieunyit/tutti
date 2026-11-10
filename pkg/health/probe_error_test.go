package health

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObserveProbeCredentialProblemIsReportedButNotAnOutage(t *testing.T) {
	r := NewRegistry(Config{FailureThreshold: 2, OpenDuration: 10})
	r.Register("s")
	authErr := &ProbeError{Step: "authentication", Detail: "SignatureDoesNotMatch"}
	for i := 0; i < 5; i++ {
		r.ObserveProbe("s", authErr)
	}
	st := r.Snapshot()[0]
	require.True(t, st.Available, "an authentication problem never opens the breaker")
	require.Equal(t, "authentication: SignatureDoesNotMatch", st.LastError)
	require.Zero(t, st.ConsecutiveFailures)
	// real traffic succeeding does not hide the probe problem
	r.Observe("s", nil)
	require.Equal(t, "authentication: SignatureDoesNotMatch", r.Snapshot()[0].LastError)
	// the next clean probe clears it
	r.ObserveProbe("s", nil)
	require.Empty(t, r.Snapshot()[0].LastError)
}

func TestObserveProbeUnavailableStepsOpenTheBreaker(t *testing.T) {
	r := NewRegistry(Config{FailureThreshold: 2, OpenDuration: 10})
	tcp := &ProbeError{Step: "dns-tcp", Detail: "connection refused", Unavailable: true}
	r.ObserveProbe("s", tcp)
	r.ObserveProbe("s", tcp)
	st := r.Snapshot()[0]
	require.False(t, st.Available)
	require.Equal(t, "dns-tcp: connection refused", st.LastError)
	require.Equal(t, Failure, Classify(tcp))
	require.Equal(t, Success, Classify(&ProbeError{Step: "authentication", Detail: "x"}))
	require.Equal(t, Failure, Classify(errors.Join(errors.New("x"), tcp)))
}
