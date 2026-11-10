// Copyright 2026 hieunyit
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package storagetest implements the "Test connection" checks of a storage with its connection account
// (docs/tutti/scale-plan.md, 2.4). The checks use plain SigV4-signed HTTP requests (no retries, no SDK
// behaviour), so every S3 error code and status reaches the report unchanged.
package storagetest

import (
	"context"
	"crypto/md5" //nolint:gosec // Content-MD5 is required by the S3 lifecycle API, not used for security
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/s3"
)

// Status of one step.
type Status string

const (
	Pass Status = "pass"
	Fail Status = "fail"
	Warn Status = "warn"
	Skip Status = "skip"
)

// Step IDs.
const (
	StepDNSTCP     = "dns-tcp"
	StepTLS        = "tls"
	StepEndpoint   = "endpoint"
	StepClock      = "clock-skew"
	StepAuth       = "authentication"
	StepRoot       = "root-account"
	StepRegion     = "region"
	StepCreate     = "create-bucket"
	StepWrite      = "write"
	StepRead       = "read"
	StepList       = "list"
	StepMultipart  = "multipart"
	StepVersioning = "versioning"
	StepFeatures   = "features"
	StepDelete     = "delete"
	StepExisting   = "existing-buckets"
	StepThroughput = "throughput"
)

// DefaultProbePrefix is the default prefix of the temporary probe bucket.
const DefaultProbePrefix = "tutti-probe-"

// ExpiryWarning is the remaining certificate validity below which the TLS step warns.
const ExpiryWarning = 14 * 24 * time.Hour

// Step is the outcome of one check.
type Step struct {
	ID                string `json:"id"`
	Number            int    `json:"number"`
	Name              string `json:"name"`
	Status            Status `json:"status"`
	Call              string `json:"call"`
	LatencyMs         int64  `json:"latencyMs"`
	Message           string `json:"message,omitempty"`
	MissingPermission string `json:"missingPermission,omitempty"`
	Hint              string `json:"hint,omitempty"`
	// Code is the S3 error code of a failed call, if any.
	Code string `json:"code,omitempty"`
}

// Result is the outcome of a test run.
type Result struct {
	TestedAt    time.Time `json:"testedAt"`
	DurationMs  int64     `json:"durationMs"`
	OK          bool      `json:"ok"`
	HasWarnings bool      `json:"hasWarnings"`
	Steps       []Step    `json:"steps"`
	ProbeBucket string    `json:"probeBucket,omitempty"`
	Leftovers   []string  `json:"leftovers,omitempty"`
	AccessKeyID string    `json:"accessKeyId,omitempty"`
	// Kind is "full" or "periodic" (steps 1-5 only).
	Kind string `json:"kind"`
}

// FirstFailure returns the first failed step.
func (r *Result) FirstFailure() (Step, bool) {
	for _, s := range r.Steps {
		if s.Status == Fail {
			return s, true
		}
	}
	return Step{}, false
}

// Failures returns a one-line summary of all failed steps ("" if none).
func (r *Result) Failures() string {
	var parts []string
	for _, s := range r.Steps {
		if s.Status == Fail {
			parts = append(parts, fmt.Sprintf("step %d %s: %s", s.Number, s.ID, s.Message))
		}
	}
	return strings.Join(parts, "; ")
}

// Features selects the optional checks.
type Features struct {
	Versioning, Tagging, ACL, Lifecycle, Policy bool
}

// Options configure a test run.
type Options struct {
	Address s3.StorageAddress
	Cred    s3.CredentialsV4
	// ProbeBucketPrefix of the temporary bucket. Default DefaultProbePrefix.
	ProbeBucketPrefix string
	// ProbeBucket is an existing bucket to use instead of creating one. It is never deleted or reconfigured.
	ProbeBucket string
	// Features to check. Nil: versioning if the provider supports it, nothing else.
	Features *Features
	// Throughput enables the optional step 17.
	Throughput bool
	// ExistingBuckets are the buckets Tutti users reference on this storage (step 16).
	ExistingBuckets []string
	// MatchBucket selects, among the buckets the account can see, the ones that access policies of users of this
	// storage allow by glob; they are checked in step 16 as well.
	MatchBucket func(bucket string) bool
	// MaxStep stops after this step number (5 for the periodic check). 0 means all.
	MaxStep int
	// RootCAs for the TLS step and all requests (default: system roots).
	RootCAs *x509.CertPool
	// Now is the clock (tests).
	Now func() time.Time
}

type step struct {
	num  int
	id   string
	name string
	call string
}

var steps = map[string]step{
	StepDNSTCP:     {1, StepDNSTCP, "DNS + TCP", "resolve host, connect to port"},
	StepTLS:        {2, StepTLS, "TLS", "TLS handshake"},
	StepEndpoint:   {3, StepEndpoint, "S3 endpoint", "GET / (unauthenticated)"},
	StepClock:      {4, StepClock, "Clock skew", "Date header of the response"},
	StepAuth:       {5, StepAuth, "Authentication", "ListBuckets"},
	StepRoot:       {6, StepRoot, "Root account", "owner of ListBuckets"},
	StepRegion:     {7, StepRegion, "Region", "GetBucketLocation"},
	StepCreate:     {8, StepCreate, "Create bucket", "CreateBucket"},
	StepWrite:      {9, StepWrite, "Write", "PutObject"},
	StepRead:       {10, StepRead, "Read", "GetObject, HeadObject"},
	StepList:       {11, StepList, "List", "ListObjectsV2"},
	StepMultipart:  {12, StepMultipart, "Multipart", "CreateMultipartUpload, UploadPart (5 MiB), ListParts, ListMultipartUploads, AbortMultipartUpload"},
	StepVersioning: {13, StepVersioning, "Versioning", "GetBucketVersioning, PutBucketVersioning, PutObject x2, ListObjectVersions, DeleteObject (version)"},
	StepFeatures:   {14, StepFeatures, "Feature checks", ""},
	StepDelete:     {15, StepDelete, "Delete", "DeleteObject, DeleteBucket"},
	StepExisting:   {16, StepExisting, "Existing buckets", "HeadBucket, ListObjectsV2 max-keys=1"},
	StepThroughput: {17, StepThroughput, "Throughput", "3 x PUT/GET of 1 MiB"},
}

// call to the S3 action that permits it.
var callAction = map[string]string{
	"ListBuckets": "s3:ListAllMyBuckets", "GetBucketLocation": "s3:GetBucketLocation", "CreateBucket": "s3:CreateBucket",
	"DeleteBucket": "s3:DeleteBucket", "PutObject": "s3:PutObject", "GetObject": "s3:GetObject", "HeadObject": "s3:GetObject",
	"HeadBucket": "s3:ListBucket", "ListObjectsV2": "s3:ListBucket", "ListObjectVersions": "s3:ListBucketVersions",
	"CreateMultipartUpload": "s3:PutObject", "UploadPart": "s3:PutObject", "ListParts": "s3:ListMultipartUploadParts",
	"ListMultipartUploads": "s3:ListBucketMultipartUploads", "AbortMultipartUpload": "s3:AbortMultipartUpload",
	"GetBucketVersioning": "s3:GetBucketVersioning", "PutBucketVersioning": "s3:PutBucketVersioning",
	"DeleteObject": "s3:DeleteObject", "DeleteObjectVersion": "s3:DeleteObjectVersion",
	"GetObjectTagging": "s3:GetObjectTagging", "PutObjectTagging": "s3:PutObjectTagging",
	"GetBucketTagging": "s3:GetBucketTagging", "PutBucketTagging": "s3:PutBucketTagging",
	"GetObjectAcl": "s3:GetObjectAcl", "PutObjectAcl": "s3:PutObjectAcl", "GetBucketAcl": "s3:GetBucketAcl", "PutBucketAcl": "s3:PutBucketAcl",
	"GetBucketLifecycleConfiguration": "s3:GetLifecycleConfiguration", "PutBucketLifecycleConfiguration": "s3:PutLifecycleConfiguration",
	"GetBucketPolicy": "s3:GetBucketPolicy", "PutBucketPolicy": "s3:PutBucketPolicy", "DeleteBucketPolicy": "s3:DeleteBucketPolicy",
}

// callError is a failed S3 call.
type callError struct {
	Call    string
	Status  int
	Code    string
	Message string
	// Transport is set for network/TLS failures (no HTTP response).
	Transport error
}

func (e *callError) Error() string {
	if e.Transport != nil {
		return e.Call + ": " + e.Transport.Error()
	}
	if e.Message != "" {
		return fmt.Sprintf("%s: %s: %s", e.Call, e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Call, e.Code, e.Status)
}

func (e *callError) denied() bool {
	switch e.Code {
	case "AccessDenied", "AllAccessDisabled", "Forbidden", "AccountProblem", "NotSignedUp", "UnauthorizedAccess":
		return true
	}
	return e.Code == "" && e.Status == http.StatusForbidden
}

func (e *callError) notImplemented() bool {
	return e.Code == "NotImplemented" || e.Status == http.StatusNotImplemented || e.Code == "XNotImplemented"
}

type s3Error struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
	Region  string `xml:"Region"`
}

type response struct {
	status  int
	header  http.Header
	body    []byte
	latency time.Duration
}

type runner struct {
	opts    Options
	client  *http.Client
	base    *url.URL
	region  string
	signer  *v4.Signer
	res     *Result
	now     func() time.Time
	timeout time.Duration

	// probe state
	bucket      string
	ownsBucket  bool
	keys        []string
	upload      string
	versioned   bool
	bucketReady bool
	runID       string
	// results needed by later steps
	owner      ownerInfo
	buckets    []string
	regionDone bool
}

type ownerInfo struct{ ID, Name string }

// Run executes the checks and returns the report. It does not return an error: transport problems are steps.
func Run(ctx context.Context, opts Options) *Result {
	r := newRunner(opts)
	r.res.Kind = "full"
	if opts.MaxStep > 0 && opts.MaxStep < 17 {
		r.res.Kind = "periodic"
	}
	start := r.now()
	r.run(ctx)
	r.res.DurationMs = r.now().Sub(start).Milliseconds()
	r.res.TestedAt = start.UTC()
	sort.SliceStable(r.res.Steps, func(i, j int) bool { return r.res.Steps[i].Number < r.res.Steps[j].Number })
	r.res.OK = true
	for _, s := range r.res.Steps {
		switch s.Status {
		case Fail:
			r.res.OK = false
		case Warn:
			r.res.HasWarnings = true
		}
	}
	if len(r.res.Leftovers) > 0 {
		r.res.HasWarnings = true
	}
	return r.res
}

// Probe runs steps 1-5 and returns a *health.ProbeError naming the first failed step (e.g. "authentication:
// SignatureDoesNotMatch"). Host, TLS and endpoint failures are marked Unavailable (they open the breaker), authentication
// and clock failures are only reported.
func Probe(ctx context.Context, opts Options) error {
	opts.MaxStep = 5
	res := Run(ctx, opts)
	if s, ok := res.FirstFailure(); ok {
		detail := s.Code
		if detail == "" {
			detail = s.Message
		}
		return &health.ProbeError{Step: s.ID, Detail: detail, Unavailable: s.ID == StepDNSTCP || s.ID == StepTLS || s.ID == StepEndpoint}
	}
	return nil
}

func newRunner(opts Options) *runner {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	timeout := opts.Address.HttpTimeout
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	r := &runner{opts: opts, now: opts.Now, timeout: timeout, signer: v4.NewSigner(func(o *v4.SignerOptions) { o.DisableURIPathEscaping = true }),
		res: &Result{AccessKeyID: opts.Cred.AccessKeyID}, runID: randomHex(4)}
	r.region = opts.Address.DefaultRegion
	if r.region == "" {
		r.region = "us-east-1"
	}
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: timeout}).DialContext,
		TLSClientConfig:     &tls.Config{RootCAs: opts.RootCAs, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
		DisableCompression:  true,
	}
	r.client = &http.Client{Transport: tr, Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return r
}

func (r *runner) add(s Step) {
	d := steps[s.ID]
	if s.Number == 0 {
		s.Number = d.num
	}
	if s.Name == "" {
		s.Name = d.name
	}
	if s.Call == "" {
		s.Call = d.call
	}
	r.res.Steps = append(r.res.Steps, s)
}

func (r *runner) skip(id, why string) { r.add(Step{ID: id, Status: Skip, Message: why}) }

func (r *runner) pass(id string, lat time.Duration, msg string) {
	r.add(Step{ID: id, Status: Pass, LatencyMs: lat.Milliseconds(), Message: msg})
}

func (r *runner) failCall(id string, lat time.Duration, err error, hint string) {
	s := Step{ID: id, Status: Fail, LatencyMs: lat.Milliseconds(), Message: err.Error(), Hint: hint}
	var ce *callError
	if errors.As(err, &ce) {
		s.Code = ce.Code
		s.Message = ce.Error()
		if ce.Transport == nil {
			s.Message = describe(ce)
		}
		if ce.denied() {
			s.MissingPermission = callAction[ce.Call]
			if s.Hint == "" && s.MissingPermission != "" {
				s.Hint = "Grant " + s.MissingPermission + " to the connection account (see docs/tutti/storage-accounts.md)."
			}
		}
	}
	r.add(s)
}

func describe(ce *callError) string {
	if ce.Message != "" {
		return fmt.Sprintf("%s failed: %s: %s", ce.Call, ce.Code, ce.Message)
	}
	return fmt.Sprintf("%s failed: %s (HTTP %d)", ce.Call, ce.Code, ce.Status)
}

func (r *runner) active(n int) bool { return r.opts.MaxStep == 0 || n <= r.opts.MaxStep }

func (r *runner) run(ctx context.Context) {
	u, err := url.Parse(r.opts.Address.Address)
	if err != nil || u.Host == "" {
		r.add(Step{ID: StepDNSTCP, Status: Fail, Message: "invalid storage address " + strconv.Quote(r.opts.Address.Address)})
		r.skipFrom(2, "address is invalid")
		return
	}
	r.base = u
	if !r.step1(ctx, u) {
		r.skipFrom(2, "host is not reachable")
		return
	}
	if !r.step2(ctx, u) {
		r.skipFrom(3, "TLS handshake failed")
		return
	}
	endpointOK := r.step3and4(ctx)
	if !endpointOK {
		r.skipFrom(5, "endpoint is not an S3 service")
		return
	}
	if !r.step5(ctx) {
		r.skipFrom(6, "authentication failed")
		return
	}
	if !r.active(6) {
		return
	}
	r.step6()
	r.prepareBucket(ctx)
	r.probeSteps(ctx)
	r.cleanup(ctx)
	r.step16(ctx)
	r.step17(ctx)
}

func (r *runner) skipFrom(n int, why string) {
	ids := make([]step, 0, len(steps))
	for _, s := range steps {
		ids = append(ids, s)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].num < ids[j].num })
	for _, s := range ids {
		if s.num >= n && r.active(s.num) && s.id != StepThroughput {
			if s.id == StepFeatures || s.id == StepVersioning {
				continue
			}
			r.skip(s.id, why)
		}
	}
}

// ---- steps 1-4

func (r *runner) step1(ctx context.Context, u *url.URL) bool {
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(cctx, host)
	if err != nil {
		r.add(Step{ID: StepDNSTCP, Status: Fail, LatencyMs: time.Since(start).Milliseconds(),
			Message: fmt.Sprintf("Cannot reach %s:%s (DNS lookup failed: %v)", host, port, unwrapNet(err)),
			Hint:    "Check the host name, the DNS of the Tutti server and the address of the storage."})
		return false
	}
	var d net.Dialer
	conn, err := d.DialContext(cctx, "tcp", net.JoinHostPort(addrs[0], port))
	if err != nil {
		r.add(Step{ID: StepDNSTCP, Status: Fail, LatencyMs: time.Since(start).Milliseconds(),
			Message: fmt.Sprintf("Cannot reach %s:%s (%s)", host, port, netReason(err)),
			Hint:    "Check that the storage listens on this port and that no firewall blocks the Tutti server."})
		return false
	}
	_ = conn.Close()
	r.pass(StepDNSTCP, time.Since(start), fmt.Sprintf("connected to %s:%s", host, port))
	return true
}

func unwrapNet(err error) error {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return errors.New(dnsErr.Err)
	}
	return err
}

func netReason(err error) string {
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()):
		return "timeout"
	case strings.Contains(err.Error(), "refused"):
		return "connection refused"
	default:
		return err.Error()
	}
}

func (r *runner) step2(ctx context.Context, u *url.URL) bool {
	if u.Scheme != "https" {
		r.skip(StepTLS, "plain HTTP endpoint (no TLS)")
		return true
	}
	if !r.active(2) {
		return true
	}
	host, port := u.Hostname(), u.Port()
	if port == "" {
		port = "443"
	}
	start := time.Now()
	cfg := &tls.Config{ServerName: host, RootCAs: r.opts.RootCAs, MinVersion: tls.VersionTLS12}
	d := &net.Dialer{Timeout: r.timeout}
	conn, err := tls.DialWithDialer(d, "tcp", net.JoinHostPort(host, port), cfg)
	if err != nil {
		step := Step{ID: StepTLS, Status: Fail, LatencyMs: time.Since(start).Milliseconds(), Message: "TLS handshake failed: " + err.Error()}
		step.Hint = "Fix the certificate of the storage."
		var unk x509.UnknownAuthorityError
		var hostErr x509.HostnameError
		var certInv x509.CertificateInvalidError
		switch {
		case errors.As(err, &unk):
			step.Hint = "The certificate is not signed by a trusted CA: a custom CA is required (add the CA certificate to the trust store of the Tutti server)."
		case errors.As(err, &hostErr):
			step.Hint = "The certificate is not valid for " + host + ": use the host name from the certificate or reissue it."
		case errors.As(err, &certInv) && certInv.Reason == x509.Expired:
			step.Hint = "The certificate has expired: renew it."
		}
		if exp := peekExpiry(d, host, port); !exp.IsZero() {
			step.Message += " (certificate expires " + exp.UTC().Format(time.RFC3339) + ")"
		}
		r.add(step)
		return false
	}
	defer conn.Close()
	state := conn.ConnectionState()
	var notAfter time.Time
	if len(state.PeerCertificates) > 0 {
		notAfter = state.PeerCertificates[0].NotAfter
	}
	remaining := notAfter.Sub(r.now())
	if !notAfter.IsZero() && remaining < ExpiryWarning {
		r.add(Step{ID: StepTLS, Status: Warn, LatencyMs: time.Since(start).Milliseconds(),
			Message: fmt.Sprintf("certificate expires %s (in %d days)", notAfter.UTC().Format(time.RFC3339), int(remaining.Hours()/24)),
			Hint:    "Renew the certificate of the storage soon."})
		return true
	}
	r.pass(StepTLS, time.Since(start), "valid certificate, expires "+notAfter.UTC().Format("2006-01-02"))
	return true
}

func peekExpiry(d *net.Dialer, host, port string) time.Time {
	conn, err := tls.DialWithDialer(d, "tcp", net.JoinHostPort(host, port), &tls.Config{ServerName: host, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}) //nolint:gosec // only reads the expiry date for the report
	if err != nil {
		return time.Time{}
	}
	defer conn.Close()
	if certs := conn.ConnectionState().PeerCertificates; len(certs) > 0 {
		return certs[0].NotAfter
	}
	return time.Time{}
}

func looksLikeS3(resp *response) bool {
	ct := strings.ToLower(resp.header.Get("Content-Type"))
	body := strings.TrimSpace(string(resp.body))
	if strings.Contains(ct, "html") || strings.HasPrefix(strings.ToLower(body), "<!doctype html") || strings.HasPrefix(strings.ToLower(body), "<html") {
		return false
	}
	if strings.HasPrefix(body, "<?xml") || strings.HasPrefix(body, "<Error") || strings.HasPrefix(body, "<ListAllMyBucketsResult") {
		return true
	}
	return false
}

func (r *runner) step3and4(ctx context.Context) bool {
	if !r.active(3) {
		return true
	}
	resp, err := r.raw(ctx, "GET", "/", nil, nil, nil, "", false)
	if err != nil {
		r.add(Step{ID: StepEndpoint, Status: Fail, Message: "Cannot talk to the endpoint: " + transportMessage(err),
			Hint: "Check the address and the scheme (http/https)."})
		r.skip(StepClock, "no response from the endpoint")
		return false
	}
	ctype := resp.header.Get("Content-Type")
	if resp.status >= 300 && resp.status < 400 || !looksLikeS3(resp) {
		r.add(Step{ID: StepEndpoint, Status: Fail, LatencyMs: resp.latency.Milliseconds(),
			Message: fmt.Sprintf("Endpoint does not look like S3 (got %d/%s)", resp.status, ctype),
			Hint:    "Use the S3 API address of the storage, not a console, web UI, redirect or reverse proxy page."})
		r.skip(StepClock, "endpoint is not S3")
		return false
	}
	r.pass(StepEndpoint, resp.latency, fmt.Sprintf("S3 response (HTTP %d, %s)", resp.status, ctype))
	r.step4(resp)
	return true
}

func (r *runner) step4(resp *response) {
	if !r.active(4) {
		return
	}
	h := resp.header.Get("Date")
	if h == "" {
		r.add(Step{ID: StepClock, Status: Warn, Message: "the storage sent no Date header: clock skew cannot be checked"})
		return
	}
	t, err := http.ParseTime(h)
	if err != nil {
		r.add(Step{ID: StepClock, Status: Warn, Message: "unparsable Date header " + strconv.Quote(h)})
		return
	}
	skew := r.now().Sub(t)
	abs := skew
	if abs < 0 {
		abs = -abs
	}
	msg := fmt.Sprintf("Clock differs by %ds", int(skew.Seconds()))
	switch {
	case abs >= 5*time.Minute:
		r.add(Step{ID: StepClock, Status: Fail, Message: msg + ": SigV4 will fail", Hint: "Synchronize the clocks (NTP) of the Tutti server and the storage."})
	case abs > time.Minute:
		r.add(Step{ID: StepClock, Status: Warn, Message: msg + ": SigV4 fails above 5 minutes", Hint: "Synchronize the clocks (NTP) of the Tutti server and the storage."})
	default:
		r.pass(StepClock, 0, msg)
	}
}

func transportMessage(err error) string {
	var ce *callError
	if errors.As(err, &ce) && ce.Transport != nil {
		return ce.Transport.Error()
	}
	return err.Error()
}

// ---- step 5, 6

var authHints = map[string]string{
	"InvalidAccessKeyId":                "The access key id is unknown to the storage: check the id of the connection account.",
	"SignatureDoesNotMatch":             "The secret key is wrong for this access key id (or the region/clock differs): re-enter the secret.",
	"AccessDenied":                      "The account is disabled or has no s3:ListAllMyBuckets permission.",
	"AllAccessDisabled":                 "The account or its access key is disabled on the storage.",
	"InvalidToken":                      "The storage requires a session token: use a long-term access key.",
	"ExpiredToken":                      "The credentials have expired.",
	"RequestTimeTooSkewed":              "Synchronize the clocks (NTP) of the Tutti server and the storage.",
	"AuthorizationHeaderMalformed":      "The signing region differs from the region of the storage: set defaultRegion to the region the storage reports.",
	"InvalidAccessKeyIdFormat":          "The access key id has an invalid format.",
	"AccountProblem":                    "The account has a problem on the storage side (suspended?).",
	"XMinioAdminAccessDenied":           "The account is not allowed to use this API.",
	"XAmzContentSHA256Mismatch":         "A proxy between Tutti and the storage altered the request.",
	"AuthorizationQueryParametersError": "The request signature was rejected.",
}

func (r *runner) step5(ctx context.Context) bool {
	if !r.active(5) {
		return true
	}
	resp, err := r.signed(ctx, "ListBuckets", "GET", "", "", nil, nil, nil)
	if err != nil {
		hint := ""
		var ce *callError
		if errors.As(err, &ce) {
			hint = authHints[ce.Code]
			if ce.Transport != nil {
				hint = "The storage could not be reached."
			}
		}
		r.failCall(StepAuth, latencyOf(resp), err, hint)
		return false
	}
	var out struct {
		Owner struct {
			ID          string `xml:"ID"`
			DisplayName string `xml:"DisplayName"`
		} `xml:"Owner"`
		Buckets []struct {
			Name string `xml:"Name"`
		} `xml:"Buckets>Bucket"`
	}
	_ = xml.Unmarshal(resp.body, &out)
	r.owner = ownerInfo{ID: out.Owner.ID, Name: out.Owner.DisplayName}
	for _, b := range out.Buckets {
		r.buckets = append(r.buckets, b.Name)
	}
	r.pass(StepAuth, resp.latency, fmt.Sprintf("authenticated as %s (%d bucket(s) visible)", r.opts.Cred.AccessKeyID, len(r.buckets)))
	return true
}

func latencyOf(resp *response) time.Duration {
	if resp == nil {
		return 0
	}
	return resp.latency
}

var rootNames = map[string]bool{"minioadmin": true, "root": true, "admin": true, "rgwadmin": true}

// step6 is a heuristic: it only looks at well-known administrator names (access key, owner id and owner display name
// reported by ListBuckets). MinIO reports the same fixed owner ("minio") for every user, so that value is ignored.
func (r *runner) step6() {
	if !r.active(6) {
		return
	}
	var why string
	switch {
	case rootNames[strings.ToLower(r.opts.Cred.AccessKeyID)]:
		why = "the access key id " + strconv.Quote(r.opts.Cred.AccessKeyID) + " is a well-known administrator name"
	case rootNames[strings.ToLower(r.owner.ID)] || rootNames[strings.ToLower(r.owner.Name)]:
		why = "the storage reports an administrator as owner (" + strconv.Quote(r.owner.Name) + ")"
	}
	if why != "" {
		r.add(Step{ID: StepRoot, Status: Warn, Message: "Looks like a root/admin account: " + why,
			Hint: "Use a dedicated account (e.g. tutti-svc) with the policy from docs/tutti/storage-accounts.md."})
		return
	}
	r.pass(StepRoot, 0, "no root/admin indicators found (heuristic: key name and owner only)")
}

// ---- probe bucket and steps 7-14

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ValidProbePrefix checks a probe bucket prefix.
func ValidProbePrefix(p string) error {
	if len(p) > 50 {
		return errors.New("probe bucket prefix must be at most 50 characters")
	}
	for _, c := range p {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '.') {
			return errors.New("probe bucket prefix may contain only lowercase letters, digits, '-' and '.'")
		}
	}
	return nil
}

func (r *runner) features() Features {
	if r.opts.Features != nil {
		return *r.opts.Features
	}
	return Features{Versioning: r.opts.Address.Provider.SupportsRetainVersionID()}
}

// prepareBucket does step 8 (or selects the existing probe bucket) and step 7 when possible.
func (r *runner) prepareBucket(ctx context.Context) {
	if r.opts.ProbeBucket != "" {
		r.bucket, r.ownsBucket, r.bucketReady = r.opts.ProbeBucket, false, true
		r.res.ProbeBucket = r.bucket
		r.skip(StepCreate, "using the existing bucket "+r.bucket+" chosen by the admin")
		r.step7(ctx)
		return
	}
	if !r.active(8) {
		return
	}
	prefix := r.opts.ProbeBucketPrefix
	if prefix == "" {
		prefix = DefaultProbePrefix
	}
	if err := ValidProbePrefix(prefix); err != nil {
		r.add(Step{ID: StepCreate, Status: Fail, Message: err.Error()})
		return
	}
	name := prefix + randomHex(4)
	var body []byte
	if r.region != "us-east-1" {
		body = []byte(`<CreateBucketConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><LocationConstraint>` + r.region + `</LocationConstraint></CreateBucketConfiguration>`)
	}
	resp, err := r.signed(ctx, "CreateBucket", "PUT", name, "", nil, nil, body)
	if err != nil {
		hint := "If creating buckets through Tutti is intentionally not allowed, select an existing bucket for the test (probeBucket)."
		var ce *callError
		if errors.As(err, &ce) && ce.denied() && prefix == DefaultProbePrefix {
			hint += " If the account policy is limited to a bucket prefix, use a probe prefix inside it (probeBucketPrefix, e.g. app-tutti-probe-)."
		}
		r.failCall(StepCreate, latencyOf(resp), err, hint)
		r.step7(ctx)
		return
	}
	r.bucket, r.ownsBucket, r.bucketReady = name, true, true
	r.res.ProbeBucket = name
	r.pass(StepCreate, resp.latency, "created "+name)
	r.step7(ctx)
}

func (r *runner) step7(ctx context.Context) {
	if r.regionDone || !r.active(7) {
		return
	}
	r.regionDone = true
	bucket := r.bucket
	if bucket == "" && len(r.buckets) > 0 {
		bucket = r.buckets[0]
	}
	if bucket == "" {
		r.skip(StepRegion, "no bucket available to ask for the region")
		return
	}
	resp, err := r.signed(ctx, "GetBucketLocation", "GET", bucket, "", url.Values{"location": {""}}, nil, nil)
	if err != nil {
		r.failCall(StepRegion, latencyOf(resp), err, "")
		return
	}
	var loc struct {
		V string `xml:",chardata"`
	}
	_ = xml.Unmarshal(resp.body, &loc)
	got := strings.TrimSpace(loc.V)
	if got == "" {
		got = "us-east-1"
	}
	want := r.opts.Address.DefaultRegion
	if want == "" {
		r.pass(StepRegion, resp.latency, "storage reports region "+got+" (defaultRegion not set)")
		return
	}
	if got != want {
		r.add(Step{ID: StepRegion, Status: Warn, LatencyMs: resp.latency.Milliseconds(),
			Message: fmt.Sprintf("Storage reports region %s, defaultRegion is %s", got, want),
			Hint:    "Set defaultRegion to " + got + "."})
		return
	}
	r.pass(StepRegion, resp.latency, "region "+got)
}

func (r *runner) probeKey(name string) string {
	return r.runPrefix() + name
}

func (r *runner) runPrefix() string { return "tutti-probe/" + r.runID + "/" }

const probeBody = "tutti connection test"

func (r *runner) probeSteps(ctx context.Context) {
	if !r.bucketReady {
		for _, id := range []string{StepWrite, StepRead, StepList, StepMultipart} {
			if r.active(steps[id].num) {
				r.skip(id, "no probe bucket (bucket creation failed or was not run)")
			}
		}
		if r.active(13) && r.features().Versioning {
			r.skip(StepVersioning, "no probe bucket")
		}
		if r.active(14) {
			r.skip(StepFeatures, "no probe bucket")
		}
		return
	}
	key := r.probeKey("obj")
	// 9 write
	wrote := false
	if r.active(9) {
		resp, err := r.signed(ctx, "PutObject", "PUT", r.bucket, key, nil, map[string]string{"Content-Type": "text/plain"}, []byte(probeBody))
		if err != nil {
			r.failCall(StepWrite, latencyOf(resp), err, "")
		} else {
			wrote = true
			r.keys = append(r.keys, key)
			r.pass(StepWrite, resp.latency, "wrote "+strconv.Itoa(len(probeBody))+" bytes")
		}
	}
	// 10 read
	if r.active(10) {
		if !wrote {
			r.skip(StepRead, "write failed")
		} else {
			start := time.Now()
			resp, err := r.signed(ctx, "GetObject", "GET", r.bucket, key, nil, nil, nil)
			switch {
			case err != nil:
				r.failCall(StepRead, latencyOf(resp), err, "")
			case string(resp.body) != probeBody:
				r.add(Step{ID: StepRead, Status: Fail, LatencyMs: resp.latency.Milliseconds(), Message: "GetObject returned different content than written"})
			default:
				if _, err := r.signed(ctx, "HeadObject", "HEAD", r.bucket, key, nil, nil, nil); err != nil {
					r.failCall(StepRead, time.Since(start), err, "")
				} else {
					r.pass(StepRead, time.Since(start), "content matches")
				}
			}
		}
	}
	// 11 list
	if r.active(11) {
		if !wrote {
			r.skip(StepList, "write failed")
		} else {
			resp, err := r.signed(ctx, "ListObjectsV2", "GET", r.bucket, "", url.Values{"list-type": {"2"}, "prefix": {"tutti-probe/"}}, nil, nil)
			switch {
			case err != nil:
				r.failCall(StepList, latencyOf(resp), err, "")
			case !strings.Contains(string(resp.body), key):
				r.add(Step{ID: StepList, Status: Fail, LatencyMs: resp.latency.Milliseconds(), Message: "ListObjectsV2 did not list the object that was written"})
			default:
				r.pass(StepList, resp.latency, "object listed")
			}
		}
	}
	r.multipart(ctx)
	r.versioning(ctx, key, wrote)
	r.featureChecks(ctx, key, wrote)
	r.throughput(ctx)
}

func (r *runner) multipart(ctx context.Context) {
	if !r.active(12) {
		return
	}
	start := time.Now()
	key := r.probeKey("mpu")
	resp, err := r.signed(ctx, "CreateMultipartUpload", "POST", r.bucket, key, url.Values{"uploads": {""}}, nil, nil)
	if err != nil {
		r.failCall(StepMultipart, time.Since(start), err, "")
		return
	}
	var init struct {
		UploadID string `xml:"UploadId"`
	}
	_ = xml.Unmarshal(resp.body, &init)
	if init.UploadID == "" {
		r.add(Step{ID: StepMultipart, Status: Fail, Message: "CreateMultipartUpload returned no UploadId"})
		return
	}
	r.upload = init.UploadID + "\x00" + key
	q := url.Values{"partNumber": {"1"}, "uploadId": {init.UploadID}}
	if _, err := r.signed(ctx, "UploadPart", "PUT", r.bucket, key, q, nil, make([]byte, 5<<20)); err != nil {
		r.failCall(StepMultipart, time.Since(start), err, "")
		return
	}
	resp, err = r.signed(ctx, "ListParts", "GET", r.bucket, key, url.Values{"uploadId": {init.UploadID}}, nil, nil)
	if err != nil {
		r.failCall(StepMultipart, time.Since(start), err, "")
		return
	}
	if !strings.Contains(string(resp.body), "<PartNumber>1</PartNumber>") {
		r.add(Step{ID: StepMultipart, Status: Fail, Message: "ListParts did not list the uploaded part"})
		return
	}
	resp, err = r.signed(ctx, "ListMultipartUploads", "GET", r.bucket, "", url.Values{"uploads": {""}}, nil, nil)
	if err != nil {
		r.failCall(StepMultipart, time.Since(start), err, "")
		return
	}
	if !strings.Contains(string(resp.body), init.UploadID) {
		r.add(Step{ID: StepMultipart, Status: Fail, Message: "ListMultipartUploads did not list the upload in progress"})
		return
	}
	if _, err := r.signed(ctx, "AbortMultipartUpload", "DELETE", r.bucket, key, url.Values{"uploadId": {init.UploadID}}, nil, nil); err != nil {
		r.failCall(StepMultipart, time.Since(start), err, "")
		return
	}
	r.upload = ""
	r.pass(StepMultipart, time.Since(start), "created, uploaded a 5 MiB part, listed and aborted an upload")
}

func (r *runner) versioning(ctx context.Context, key string, wrote bool) {
	if !r.active(13) {
		return
	}
	if !r.features().Versioning {
		r.skip(StepVersioning, "versioned buckets are not used (provider does not support versioning or the check is disabled)")
		return
	}
	start := time.Now()
	fail := func(err error) { r.failVersioning(StepVersioning, time.Since(start), err) }
	resp, err := r.signed(ctx, "GetBucketVersioning", "GET", r.bucket, "", url.Values{"versioning": {""}}, nil, nil)
	if err != nil {
		fail(err)
		return
	}
	var config struct {
		Status string `xml:"Status"`
	}
	if err := xml.Unmarshal(resp.body, &config); err != nil {
		fail(fmt.Errorf("decode bucket versioning: %w", err))
		return
	}
	r.versioned = r.versioned || config.Status == "Enabled" || config.Status == "Suspended"
	if !r.ownsBucket {
		r.add(Step{ID: StepVersioning, Status: Warn, LatencyMs: time.Since(start).Milliseconds(),
			Message: "GetBucketVersioning works; versioning was not changed on the existing probe bucket " + r.bucket,
			Hint:    "Let Tutti create a probe bucket to check PutBucketVersioning and version deletion."})
		return
	}
	body := []byte(`<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>Enabled</Status></VersioningConfiguration>`)
	if _, err := r.signed(ctx, "PutBucketVersioning", "PUT", r.bucket, "", url.Values{"versioning": {""}}, nil, body); err != nil {
		fail(err)
		return
	}
	r.versioned = true
	vkey := r.probeKey("ver")
	var versionIDs []string
	for i := 0; i < 2; i++ {
		resp, err := r.signed(ctx, "PutObject", "PUT", r.bucket, vkey, nil, nil, []byte(fmt.Sprintf("v%d", i)))
		if err != nil {
			fail(err)
			return
		}
		versionIDs = append(versionIDs, resp.header.Get("X-Amz-Version-Id"))
	}
	resp, err = r.signed(ctx, "ListObjectVersions", "GET", r.bucket, "", url.Values{"versions": {""}, "prefix": {vkey}}, nil, nil)
	if err != nil {
		fail(err)
		return
	}
	if strings.Count(string(resp.body), "<Version>") < 2 {
		r.add(Step{ID: StepVersioning, Status: Fail, Message: "ListObjectVersions returned fewer than 2 versions of the probe object"})
		return
	}
	if versionIDs[0] != "" {
		if _, err := r.signed(ctx, "DeleteObjectVersion", "DELETE", r.bucket, vkey, url.Values{"versionId": {versionIDs[0]}}, nil, nil); err != nil {
			fail(err)
			return
		}
	}
	r.pass(StepVersioning, time.Since(start), "versioning enabled on the probe bucket, 2 versions written and listed, 1 version deleted")
}

func (r *runner) failVersioning(id string, lat time.Duration, err error) {
	var ce *callError
	if errors.As(err, &ce) && ce.notImplemented() {
		r.add(Step{ID: id, Status: Warn, LatencyMs: lat.Milliseconds(), Message: "the provider does not support versioning (" + ce.Call + ": NotImplemented)",
			Hint: "Do not use versioned buckets on this storage."})
		return
	}
	r.failCall(id, lat, err, "")
}

type featureCall struct {
	call, method, key, bucketScope string
	query                          url.Values
	headers                        map[string]string
	body                           []byte
	bucketLevel                    bool
}

func (r *runner) featureChecks(ctx context.Context, key string, wrote bool) {
	if !r.active(14) {
		return
	}
	f := r.features()
	if !f.Tagging && !f.ACL && !f.Lifecycle && !f.Policy {
		r.skip(StepFeatures, "no optional features (tagging, ACL, lifecycle, policy) enabled")
		return
	}
	type group struct {
		name  string
		calls []featureCall
	}
	var groups []group
	if f.Tagging {
		tag := []byte(`<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><TagSet><Tag><Key>tutti</Key><Value>probe</Value></Tag></TagSet></Tagging>`)
		q := url.Values{"tagging": {""}}
		groups = append(groups, group{"tagging", []featureCall{
			{call: "PutObjectTagging", method: "PUT", key: key, query: q, body: tag},
			{call: "GetObjectTagging", method: "GET", key: key, query: q},
			{call: "PutBucketTagging", method: "PUT", query: q, body: tag, bucketLevel: true},
			{call: "GetBucketTagging", method: "GET", query: q, bucketLevel: true},
		}})
	}
	if f.ACL {
		q := url.Values{"acl": {""}}
		h := map[string]string{"X-Amz-Acl": "private"}
		groups = append(groups, group{"acl", []featureCall{
			{call: "PutObjectAcl", method: "PUT", key: key, query: q, headers: h},
			{call: "GetObjectAcl", method: "GET", key: key, query: q},
			{call: "PutBucketAcl", method: "PUT", query: q, headers: h, bucketLevel: true},
			{call: "GetBucketAcl", method: "GET", query: q, bucketLevel: true},
		}})
	}
	if f.Lifecycle {
		q := url.Values{"lifecycle": {""}}
		body := []byte(`<LifecycleConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Rule><ID>tutti-probe</ID><Filter><Prefix>tutti-probe-never/</Prefix></Filter><Status>Enabled</Status><Expiration><Days>3650</Days></Expiration></Rule></LifecycleConfiguration>`)
		sum := md5.Sum(body) //nolint:gosec // required by the API
		groups = append(groups, group{"lifecycle", []featureCall{
			{call: "PutBucketLifecycleConfiguration", method: "PUT", query: q, body: body, headers: map[string]string{"Content-MD5": base64.StdEncoding.EncodeToString(sum[:])}, bucketLevel: true},
			{call: "GetBucketLifecycleConfiguration", method: "GET", query: q, bucketLevel: true},
		}})
	}
	if f.Policy {
		q := url.Values{"policy": {""}}
		body := []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetBucketLocation"],"Resource":["arn:aws:s3:::` + r.bucket + `"]}]}`)
		groups = append(groups, group{"policy", []featureCall{
			{call: "PutBucketPolicy", method: "PUT", query: q, body: body, bucketLevel: true},
			{call: "GetBucketPolicy", method: "GET", query: q, bucketLevel: true},
			{call: "DeleteBucketPolicy", method: "DELETE", query: q, bucketLevel: true},
		}})
	}
	for _, g := range groups {
		start := time.Now()
		id := StepFeatures
		st := Step{ID: id, Number: 14, Name: "Feature check: " + g.name, Status: Pass}
		var names []string
		var missing []string
		var failures []string
		for _, c := range g.calls {
			names = append(names, c.call)
			if c.bucketLevel && !r.ownsBucket {
				st.Status = Warn
				failures = append(failures, c.call+" skipped on the existing probe bucket")
				continue
			}
			if !c.bucketLevel && !wrote {
				failures = append(failures, c.call+" skipped (write failed)")
				st.Status = Fail
				continue
			}
			if _, err := r.signed(ctx, c.call, c.method, r.bucket, c.key, c.query, c.headers, c.body); err != nil {
				st.Status = Fail
				var ce *callError
				if errors.As(err, &ce) {
					failures = append(failures, describe(ce))
					if ce.denied() {
						missing = append(missing, callAction[c.call])
					}
					if st.Code == "" {
						st.Code = ce.Code
					}
				} else {
					failures = append(failures, err.Error())
				}
			}
		}
		st.Call = strings.Join(names, ", ")
		st.LatencyMs = time.Since(start).Milliseconds()
		st.MissingPermission = strings.Join(missing, ", ")
		if len(failures) > 0 {
			st.Message = strings.Join(failures, "; ")
			if st.Status == Fail {
				st.Hint = "Grant the missing permissions or disable the " + g.name + " feature for this storage."
			}
		} else {
			st.Message = g.name + " calls succeeded"
		}
		r.add(st)
		// leave the probe bucket clean for the delete step
		if r.ownsBucket {
			switch g.name {
			case "lifecycle":
				_, _ = r.signed(ctx, "DeleteBucketLifecycle", "DELETE", r.bucket, "", url.Values{"lifecycle": {""}}, nil, nil)
			case "tagging":
				_, _ = r.signed(ctx, "DeleteBucketTagging", "DELETE", r.bucket, "", url.Values{"tagging": {""}}, nil, nil)
			}
		}
	}
}

// ---- step 15 cleanup

func (r *runner) cleanup(ctx context.Context) {
	if !r.bucketReady {
		if r.active(15) {
			r.skip(StepDelete, "nothing was created")
		}
		return
	}
	if !r.active(15) {
		return
	}
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()
	start := time.Now()
	var errs []string
	var missing []string
	var firstCode string
	note := func(err error, leftover string) {
		var ce *callError
		if errors.As(err, &ce) {
			errs = append(errs, describe(ce))
			if ce.denied() {
				missing = append(missing, callAction[ce.Call])
			}
			if firstCode == "" {
				firstCode = ce.Code
			}
		} else {
			errs = append(errs, err.Error())
		}
		if leftover != "" {
			r.res.Leftovers = append(r.res.Leftovers, leftover)
		}
	}
	if r.upload != "" {
		id, key, _ := strings.Cut(r.upload, "\x00")
		if _, err := r.signed(cctx, "AbortMultipartUpload", "DELETE", r.bucket, key, url.Values{"uploadId": {id}}, nil, nil); err != nil {
			note(err, "multipart upload "+id+" of "+r.bucket+"/"+key)
		}
	}
	// DeleteObject first (the permission of the plan), then every remaining version and delete marker
	for _, k := range r.keys {
		if _, err := r.signed(cctx, "DeleteObject", "DELETE", r.bucket, k, nil, nil, nil); err != nil {
			note(err, r.bucket+"/"+k)
		}
	}

	// objects of the multipart/feature steps that are not tracked: list and remove what is left under the probe prefix
	if len(errs) == 0 {
		if resp, err := r.signed(cctx, "ListObjectsV2", "GET", r.bucket, "", url.Values{"list-type": {"2"}, "prefix": {r.runPrefix()}}, nil, nil); err == nil {
			var lo struct {
				Keys []string `xml:"Contents>Key"`
			}
			_ = xml.Unmarshal(resp.body, &lo)
			for _, k := range lo.Keys {
				if _, err := r.signed(cctx, "DeleteObject", "DELETE", r.bucket, k, nil, nil, nil); err != nil {
					note(err, r.bucket+"/"+k)
				}
			}
		}
	}
	if r.versioned {
		versions, err := r.probeVersions(cctx)
		if err != nil {
			note(err, r.bucket+"/"+r.runPrefix()+" (versions could not be listed)")
		} else {
			for _, v := range versions {
				if _, err := r.signed(cctx, "DeleteObjectVersion", "DELETE", r.bucket, v.Key, url.Values{"versionId": {v.VersionId}}, nil, nil); err != nil {
					note(err, r.bucket+"/"+v.Key+" (version "+v.VersionId+")")
				}
			}
		}
	}

	if r.ownsBucket {
		if _, err := r.signed(cctx, "DeleteBucket", "DELETE", r.bucket, "", nil, nil, nil); err != nil {
			note(err, "bucket "+r.bucket)
		}
	}
	if len(errs) > 0 {
		r.res.Leftovers = r.actualLeftovers(cctx, r.res.Leftovers)
	}
	if len(errs) > 0 {
		r.add(Step{ID: StepDelete, Status: Fail, LatencyMs: time.Since(start).Milliseconds(), Code: firstCode,
			Message:           strings.Join(errs, "; ") + ". Left behind: " + strings.Join(r.res.Leftovers, ", "),
			MissingPermission: strings.Join(dedupe(missing), ", "),
			Hint:              "Remove the leftovers manually and grant the missing permissions."})
		return
	}
	msg := "probe objects removed"
	if r.ownsBucket {
		msg = "probe objects and bucket " + r.bucket + " removed"
	}
	r.pass(StepDelete, time.Since(start), msg)
}

type probeVersion struct {
	Key, VersionId string
}

// Collect before deleting: pagination markers may name versions removed by cleanup.
func (r *runner) probeVersions(ctx context.Context) ([]probeVersion, error) {
	query := url.Values{"versions": {""}, "prefix": {r.runPrefix()}}
	var versions []probeVersion
	for {
		resp, err := r.signed(ctx, "ListObjectVersions", "GET", r.bucket, "", query, nil, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Versions    []probeVersion `xml:"Version"`
			Markers     []probeVersion `xml:"DeleteMarker"`
			Truncated   bool           `xml:"IsTruncated"`
			NextKey     string         `xml:"NextKeyMarker"`
			NextVersion string         `xml:"NextVersionIdMarker"`
		}
		if err := xml.Unmarshal(resp.body, &page); err != nil {
			return nil, fmt.Errorf("decode probe versions: %w", err)
		}
		for _, v := range append(page.Versions, page.Markers...) {
			if strings.HasPrefix(v.Key, r.runPrefix()) {
				versions = append(versions, v)
			}
		}
		if !page.Truncated {
			return versions, nil
		}
		if page.NextKey == "" || (page.NextKey == query.Get("key-marker") && page.NextVersion == query.Get("version-id-marker")) {
			return nil, errors.New("probe version listing returned a non-advancing pagination marker")
		}
		query.Set("key-marker", page.NextKey)
		query.Set("version-id-marker", page.NextVersion)
	}
}

// actualLeftovers re-checks what is really still there after failed deletions (a failed call may have been
// compensated by another one); it falls back to the failed targets if the storage cannot be listed.
func (r *runner) actualLeftovers(ctx context.Context, fallback []string) []string {
	var out []string
	if r.ownsBucket {
		if _, err := r.signed(ctx, "HeadBucket", "HEAD", r.bucket, "", nil, nil, nil); err != nil {
			var ce *callError
			if errors.As(err, &ce) && ce.Status == http.StatusNotFound {
				return nil
			}
			return fallback
		}
		out = append(out, "bucket "+r.bucket)
	}
	if r.versioned {
		versions, err := r.probeVersions(ctx)
		if err != nil {
			return fallback
		}
		for _, v := range versions {
			out = append(out, r.bucket+"/"+v.Key+" (version "+v.VersionId+")")
		}
		return out
	}
	q := url.Values{"list-type": {"2"}, "prefix": {r.runPrefix()}}
	call := "ListObjectsV2"
	resp, err := r.signed(ctx, call, "GET", r.bucket, "", q, nil, nil)
	if err != nil {
		return fallback
	}
	var lo struct {
		Keys []string `xml:"Contents>Key"`
		Vers []struct {
			Key, VersionId string
		} `xml:"Version"`
		Marks []struct {
			Key, VersionId string
		} `xml:"DeleteMarker"`
	}
	_ = xml.Unmarshal(resp.body, &lo)
	for _, k := range lo.Keys {
		out = append(out, r.bucket+"/"+k)
	}
	for _, v := range append(lo.Vers, lo.Marks...) {
		out = append(out, r.bucket+"/"+v.Key+" (version "+v.VersionId+")")
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ---- step 16, 17

func (r *runner) step16(ctx context.Context) {
	if !r.active(16) {
		return
	}
	existing := append([]string(nil), r.opts.ExistingBuckets...)
	if r.opts.MatchBucket != nil {
		for _, b := range r.buckets {
			if r.opts.MatchBucket(b) && !slices.Contains(existing, b) {
				existing = append(existing, b)
			}
		}
	}
	if len(existing) == 0 {
		r.skip(StepExisting, "no bucket of a Tutti user, routing or replication refers to this storage")
		return
	}
	start := time.Now()
	var failed, warned, missing []string
	for _, b := range existing {
		if _, err := r.signed(ctx, "HeadBucket", "HEAD", b, "", nil, nil, nil); err != nil {
			var ce *callError
			if errors.As(err, &ce) && (ce.Status == http.StatusNotFound || ce.Code == "NoSuchBucket") {
				warned = append(warned, b+" does not exist yet")
				continue
			}
			failed = append(failed, b)
			missing = append(missing, "s3:ListBucket on "+b)
			continue
		}
		if _, err := r.signed(ctx, "ListObjectsV2", "GET", b, "", url.Values{"list-type": {"2"}, "max-keys": {"1"}}, nil, nil); err != nil {
			failed = append(failed, b)
			missing = append(missing, "s3:ListBucket on "+b)
		}
	}
	switch {
	case len(failed) > 0:
		r.add(Step{ID: StepExisting, Status: Fail, LatencyMs: time.Since(start).Milliseconds(), Code: "AccessDenied",
			Message:           "Account cannot access bucket(s) " + strings.Join(failed, ", ") + " (owned by another account?)",
			MissingPermission: strings.Join(missing, ", "),
			Hint: "Add a bucket policy that grants the connection account access, e.g. " +
				`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam:::user/` + r.opts.Cred.AccessKeyID +
				`"]},"Action":["s3:*"],"Resource":["arn:aws:s3:::BUCKET","arn:aws:s3:::BUCKET/*"]}]}`})
	case len(warned) > 0:
		r.add(Step{ID: StepExisting, Status: Warn, LatencyMs: time.Since(start).Milliseconds(), Message: strings.Join(warned, "; ") + " (created on first use)"})
	default:
		r.pass(StepExisting, time.Since(start), fmt.Sprintf("%d referenced bucket(s) accessible", len(existing)))
	}
}

func (r *runner) throughput(ctx context.Context) {
	// executed while the probe bucket exists, reported in step 17
	if !r.opts.Throughput || r.opts.MaxStep != 0 {
		return
	}
	if !r.bucketReady {
		r.skip(StepThroughput, "no probe bucket")
		return
	}
	body := make([]byte, 1<<20)
	var puts, gets []time.Duration
	var key string
	for i := 0; i < 3; i++ {
		key = r.probeKey("tp" + strconv.Itoa(i))
		resp, err := r.signed(ctx, "PutObject", "PUT", r.bucket, key, nil, nil, body)
		if err != nil {
			r.add(Step{ID: StepThroughput, Status: Warn, Message: err.Error()})
			return
		}
		r.keys = append(r.keys, key)
		puts = append(puts, resp.latency)
	}
	for i := 0; i < 3; i++ {
		resp, err := r.signed(ctx, "GetObject", "GET", r.bucket, r.probeKey("tp"+strconv.Itoa(i)), nil, nil, nil)
		if err != nil {
			r.add(Step{ID: StepThroughput, Status: Warn, Message: err.Error()})
			return
		}
		gets = append(gets, resp.latency)
	}
	rate := func(ds []time.Duration) (float64, time.Duration) {
		var total time.Duration
		for _, d := range ds {
			total += d
		}
		s := append([]time.Duration(nil), ds...)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		return float64(len(ds)) / total.Seconds(), s[len(s)-1]
	}
	pr, pp := rate(puts)
	gr, gp := rate(gets)
	r.add(Step{ID: StepThroughput, Status: Pass,
		Message: fmt.Sprintf("PUT %.1f MiB/s (p95 %dms), GET %.1f MiB/s (p95 %dms)", pr, pp.Milliseconds(), gr, gp.Milliseconds())})
}

func (r *runner) step17(ctx context.Context) {
	if r.opts.Throughput && r.opts.MaxStep == 0 {
		found := false
		for _, s := range r.res.Steps {
			if s.ID == StepThroughput {
				found = true
			}
		}
		if !found {
			r.skip(StepThroughput, "not run")
		}
	}
}

// ---- HTTP

func (r *runner) reqURL(bucket, key string, q url.Values) string {
	u := *r.base
	p := "/"
	if bucket != "" {
		p += bucket
		if key != "" {
			p += "/" + key
		}
	}
	u.Path = strings.TrimSuffix(r.base.Path, "/") + p
	u.RawQuery = q.Encode()
	return u.String()
}

// raw sends one request, signed or not.
func (r *runner) raw(ctx context.Context, method, path string, q url.Values, headers map[string]string, body []byte, call string, sign bool) (*response, error) {
	u := *r.base
	u.Path = strings.TrimSuffix(r.base.Path, "/") + path
	u.RawQuery = q.Encode()
	return r.send(ctx, method, u.String(), headers, body, call, sign)
}

func (r *runner) send(ctx context.Context, method, rawURL string, headers map[string]string, body []byte, call string, sign bool) (*response, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, rawURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, &callError{Call: call, Transport: err}
	}
	req.ContentLength = int64(len(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if sign {
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:])
		req.Header.Set("X-Amz-Content-Sha256", hash)
		cred := aws.Credentials{AccessKeyID: r.opts.Cred.AccessKeyID, SecretAccessKey: r.opts.Cred.SecretAccessKey}
		if err := r.signer.SignHTTP(ctx, cred, req, hash, "s3", r.region, time.Now()); err != nil {
			return nil, &callError{Call: call, Transport: err}
		}
	}
	start := time.Now()
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, &callError{Call: call, Transport: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	out := &response{status: resp.StatusCode, header: resp.Header, body: data, latency: time.Since(start)}
	if err != nil {
		return out, &callError{Call: call, Transport: err}
	}
	return out, nil
}

// signed sends a signed request; a status >= 300 is returned as *callError together with the response.
func (r *runner) signed(ctx context.Context, call, method, bucket, key string, q url.Values, headers map[string]string, body []byte) (*response, error) {
	resp, err := r.send(ctx, method, r.reqURL(bucket, key, q), headers, body, call, true)
	// Detect versioned writes even when the optional versioning feature check is disabled.
	if resp != nil && resp.status < 300 && bucket == r.bucket && strings.HasPrefix(key, r.runPrefix()) && resp.header.Get("X-Amz-Version-Id") != "" {
		r.versioned = true
	}
	if err != nil {
		return resp, err
	}
	if resp.status >= 300 {
		ce := &callError{Call: call, Status: resp.status}
		var e s3Error
		if xml.Unmarshal(resp.body, &e) == nil {
			ce.Code, ce.Message = e.Code, strings.TrimSpace(e.Message)
		}
		if ce.Code == "" {
			ce.Code = httpCode(resp.status)
		}
		return resp, ce
	}
	return resp, nil
}

func httpCode(status int) string {
	switch status {
	case http.StatusForbidden:
		return "AccessDenied"
	case http.StatusNotFound:
		return "NotFound"
	case http.StatusNotImplemented:
		return "NotImplemented"
	case http.StatusServiceUnavailable:
		return "ServiceUnavailable"
	default:
		return "HTTP" + strconv.Itoa(status)
	}
}
