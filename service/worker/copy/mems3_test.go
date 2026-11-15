package copy

// A minimal in-process S3 server for tests and benchmarks (test-only code, never part of the worker).
// It supports HEAD/GET (ranges, If-Match), PUT object, multipart initiate/part/complete/abort, ListParts,
// ListMultipartUploads and tagging reads, plus injectable latency, per-stream and global bandwidth caps and
// faults (fail a request, drop the connection mid-body).

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type memFault struct {
	Status    int // answer with this HTTP status and an S3 error body
	Code      string
	DropAfter int64 // >= 0 with Drop: close the connection after this many body bytes
	Drop      bool
}

type memObj struct {
	data     []byte
	size     int64
	etag     string
	header   http.Header
	modified time.Time
	tagging  string
}

type memPart struct {
	etag string
	size int64
	data []byte
}

type memUpload struct {
	key       string
	id        string
	parts     map[int]*memPart
	header    http.Header
	initiated time.Time
}

type memS3 struct {
	mu      sync.Mutex
	objs    map[string]*memObj
	uploads map[string]*memUpload
	seq     int
	reqLog  []string
	reqN    int

	// synthetic mode: objects named "s<size>..." exist without storage and uploads are discarded (benchmarks)
	synthetic bool

	Latency   time.Duration
	StreamBps int64 // per request body cap, bytes/s (0 = unlimited)
	totalLim  *rate.Limiter
	Hook      func(n int, r *http.Request) *memFault

	srv        *httptest.Server
	putBytes   atomic.Int64
	partsPut   atomic.Int64
	curParts   atomic.Int64
	maxParts   atomic.Int64
	gets       atomic.Int64
	rangedGets atomic.Int64
}

func newMemS3() *memS3 {
	f := &memS3{objs: map[string]*memObj{}, uploads: map[string]*memUpload{}}
	f.srv = httptest.NewServer(f)
	return f
}

func (f *memS3) URL() string { return f.srv.URL }
func (f *memS3) Close()      { f.srv.CloseClientConnections(); f.srv.Close() }

// SetTotalBps caps the total body throughput of all requests together.
func (f *memS3) SetTotalBps(bps int64) {
	if bps <= 0 {
		f.totalLim = nil
		return
	}
	f.totalLim = rate.NewLimiter(rate.Limit(bps), 256<<10)
}

func (f *memS3) Put(bucketKey string, data []byte, header http.Header) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objs[bucketKey] = newFakeObj(data, header)
}

func newFakeObj(data []byte, header http.Header) *memObj {
	sum := md5.Sum(data)
	h := http.Header{}
	for k, v := range header {
		h[k] = v
	}
	return &memObj{data: data, size: int64(len(data)), etag: hex.EncodeToString(sum[:]), header: h, modified: time.Now().UTC().Truncate(time.Second)}
}

func (f *memS3) Get(bucketKey string) (*memObj, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objs[bucketKey]
	return o, ok
}

func (f *memS3) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.reqLog...)
}

func (f *memS3) CountRequests(prefix string) int {
	n := 0
	for _, r := range f.Requests() {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (f *memS3) OpenUploads() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for id := range f.uploads {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Age moves the initiation time of an upload into the past.
func (f *memS3) Age(uploadID string, age time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u := f.uploads[uploadID]; u != nil {
		u.initiated = time.Now().Add(-age)
	}
}

// SeedUpload creates an upload with the given parts (data per part number) and returns its id.
func (f *memS3) SeedUpload(bucketKey string, header http.Header, parts map[int][]byte) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("seed%d", f.seq)
	u := &memUpload{key: bucketKey, id: id, parts: map[int]*memPart{}, header: header, initiated: time.Now()}
	for n, d := range parts {
		sum := md5.Sum(d)
		u.parts[n] = &memPart{etag: hex.EncodeToString(sum[:]), size: int64(len(d)), data: d}
	}
	f.uploads[id] = u
	return id
}

func (f *memS3) DropUpload(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.uploads, id)
}

func (f *memS3) UploadParts(id string) map[int]memPart {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]memPart{}
	if u := f.uploads[id]; u != nil {
		for n, p := range u.parts {
			out[n] = *p
		}
	}
	return out
}

// CorruptPart replaces the stored bytes (and ETag) of a part, as if the destination lost data.
func (f *memS3) CorruptPart(id string, n int, newSize int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u := f.uploads[id]; u != nil && u.parts[n] != nil {
		u.parts[n].size = newSize
		u.parts[n].etag = "corrupt"
		u.parts[n].data = make([]byte, newSize)
	}
}

func s3err(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, code)
}

var synthBlock = func() []byte {
	b := make([]byte, 1<<20)
	for i := range b {
		b[i] = byte(i*31 + 7)
	}
	return b
}()

func synthSize(key string) (int64, bool) {
	base := key[strings.LastIndex(key, "/")+1:]
	if !strings.HasPrefix(base, "s") {
		return 0, false
	}
	end := strings.IndexAny(base[1:], "-_")
	digits := base[1:]
	if end >= 0 {
		digits = base[1 : 1+end]
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	return n, err == nil
}

// throttle paces n bytes of one stream.
func (f *memS3) pace(start time.Time, sent int64) {
	if f.StreamBps > 0 {
		due := start.Add(time.Duration(float64(sent) / float64(f.StreamBps) * float64(time.Second)))
		if d := time.Until(due); d > 0 {
			time.Sleep(d)
		}
	}
	if l := f.totalLim; l != nil {
		n := 64 << 10
		_ = l.WaitN(nil2ctx{}, min(n, l.Burst()))
	}
}

type nil2ctx struct{}

func (nil2ctx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (nil2ctx) Done() <-chan struct{}       { return nil }
func (nil2ctx) Err() error                  { return nil }
func (nil2ctx) Value(any) any               { return nil }

func hijackClose(w http.ResponseWriter) {
	if h, ok := w.(http.Hijacker); ok {
		if c, _, err := h.Hijack(); err == nil {
			if tc, ok := c.(*net.TCPConn); ok {
				_ = tc.SetLinger(0)
			}
			_ = c.Close()
		}
	}
}

// readBody reads the request body in 64 KiB steps, pacing it; keep keeps the bytes. A drop fault closes the
// connection after dropAfter bytes.
func (f *memS3) readBody(w http.ResponseWriter, r *http.Request, keep bool, fault *memFault) ([]byte, int64, bool) {
	var out []byte
	buf := make([]byte, 64<<10)
	var n int64
	start := time.Now()
	for {
		m, err := r.Body.Read(buf)
		if m > 0 {
			n += int64(m)
			if keep {
				out = append(out, buf[:m]...)
			}
			f.pace(start, n)
			if fault != nil && fault.Drop && n >= fault.DropAfter {
				hijackClose(w)
				return nil, n, false
			}
		}
		if err != nil {
			if err != io.EOF {
				return nil, n, false
			}
			return out, n, true
		}
	}
}

func (f *memS3) writeData(w http.ResponseWriter, data []byte, synthSize int64, fault *memFault) {
	start := time.Now()
	var sent int64
	write := func(b []byte) bool {
		for len(b) > 0 {
			c := b
			if len(c) > 64<<10 {
				c = c[:64<<10]
			}
			if fault != nil && fault.Drop && sent+int64(len(c)) > fault.DropAfter {
				c = c[:fault.DropAfter-sent]
				_, _ = w.Write(c)
				if fl, ok := w.(http.Flusher); ok {
					fl.Flush()
				}
				hijackClose(w)
				return false
			}
			if _, err := w.Write(c); err != nil {
				return false
			}
			sent += int64(len(c))
			f.pace(start, sent)
			b = b[len(c):]
		}
		return true
	}
	if data != nil {
		write(data)
		return
	}
	left := synthSize
	for left > 0 {
		c := synthBlock
		if int64(len(c)) > left {
			c = c[:left]
		}
		if !write(c) {
			return
		}
		left -= int64(len(c))
	}
}

func objHeaders(src http.Header) http.Header {
	out := http.Header{}
	for k, v := range src {
		lk := strings.ToLower(k)
		switch {
		case strings.HasPrefix(lk, "x-amz-meta-"), lk == "content-type", lk == "content-encoding", lk == "content-disposition",
			lk == "content-language", lk == "cache-control", lk == "expires", lk == "x-amz-tagging":
			out[http.CanonicalHeaderKey(k)] = v
		}
	}
	return out
}

type partsXML struct {
	XMLName xml.Name `xml:"CompleteMultipartUpload"`
	Parts   []struct {
		PartNumber int
		ETag       string
	} `xml:"Part"`
}

func (f *memS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "probe-health-test") {
		w.WriteHeader(403)
		return
	}
	q := r.URL.Query()
	f.mu.Lock()
	f.reqN++
	n := f.reqN
	f.reqLog = append(f.reqLog, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
	f.mu.Unlock()
	if f.Latency > 0 {
		time.Sleep(f.Latency)
	}
	var fault *memFault
	if f.Hook != nil {
		fault = f.Hook(n, r)
	}
	if fault != nil && fault.Status != 0 {
		io.Copy(io.Discard, r.Body)
		s3err(w, fault.Status, fault.Code)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	hasKey := strings.Contains(strings.TrimSuffix(path, "/"), "/")
	path = strings.TrimSuffix(path, "/")
	switch {
	case !hasKey && r.Method == http.MethodGet && q.Has("uploads"):
		f.listUploads(w, path, q.Get("prefix"))
	case !hasKey:
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, `<LocationConstraint>us-east-1</LocationConstraint>`)
	case r.Method == http.MethodPost && q.Has("uploads"):
		f.mu.Lock()
		f.seq++
		id := fmt.Sprintf("up%d", f.seq)
		f.uploads[id] = &memUpload{key: path, id: id, parts: map[int]*memPart{}, header: objHeaders(r.Header), initiated: time.Now()}
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		bucket, key, _ := strings.Cut(path, "/")
		fmt.Fprintf(w, `<InitiateMultipartUploadResult><Bucket>%s</Bucket><Key>%s</Key><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, bucket, key, id)
	case r.Method == http.MethodPut && q.Has("partNumber"):
		f.putPart(w, r, path, fault)
	case r.Method == http.MethodPost && q.Has("uploadId"):
		f.complete(w, r, path)
	case r.Method == http.MethodDelete && q.Has("uploadId"):
		f.mu.Lock()
		_, ok := f.uploads[q.Get("uploadId")]
		delete(f.uploads, q.Get("uploadId"))
		f.mu.Unlock()
		if !ok {
			s3err(w, 404, "NoSuchUpload")
			return
		}
		w.WriteHeader(204)
	case r.Method == http.MethodGet && q.Has("uploadId"):
		f.listParts(w, q, path)
	case r.Method == http.MethodPut:
		var data []byte
		body, nb, ok := f.readBody(w, r, !f.synthetic, fault)
		if !ok {
			return
		}
		f.putBytes.Add(nb)
		data = body
		f.mu.Lock()
		if f.synthetic {
			f.objs[path] = &memObj{size: nb, etag: "synth", header: objHeaders(r.Header), modified: time.Now()}
			w.Header().Set("ETag", `"synth"`)
		} else {
			o := newFakeObj(data, objHeaders(r.Header))
			f.objs[path] = o
			w.Header().Set("ETag", `"`+o.etag+`"`)
		}
		f.mu.Unlock()
	case r.Method == http.MethodHead || r.Method == http.MethodGet:
		f.getObject(w, r, path, q.Has("tagging"), fault)
	case r.Method == http.MethodDelete:
		f.mu.Lock()
		delete(f.objs, path)
		f.mu.Unlock()
		w.WriteHeader(204)
	default:
		io.Copy(io.Discard, r.Body)
	}
}

func (f *memS3) lookup(path string) (*memObj, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if o, ok := f.objs[path]; ok {
		return o, true
	}
	if f.synthetic {
		if sz, ok := synthSize(path); ok {
			return &memObj{size: sz, etag: "synthetic-" + strconv.FormatInt(sz, 10), header: http.Header{"Content-Type": {"application/octet-stream"}}, modified: time.Unix(1700000000, 0)}, true
		}
	}
	return nil, false
}

func (f *memS3) getObject(w http.ResponseWriter, r *http.Request, path string, tagging bool, fault *memFault) {
	o, ok := f.lookup(path)
	if !ok {
		s3err(w, 404, "NoSuchKey")
		return
	}
	if tagging {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, `<Tagging><TagSet>`+o.tagging+`</TagSet></Tagging>`)
		return
	}
	if im := r.Header.Get("If-Match"); im != "" && strings.Trim(im, `"`) != o.etag {
		s3err(w, 412, "PreconditionFailed")
		return
	}
	h := w.Header()
	for k, v := range o.header {
		h[k] = v
	}
	h.Set("ETag", `"`+o.etag+`"`)
	h.Set("Last-Modified", o.modified.UTC().Format(http.TimeFormat))
	h.Set("Accept-Ranges", "bytes")
	a, b := int64(0), o.size-1
	status := 200
	if rg := r.Header.Get("Range"); rg != "" {
		var err error
		spec := strings.TrimPrefix(rg, "bytes=")
		lo, hi, _ := strings.Cut(spec, "-")
		a, err = strconv.ParseInt(lo, 10, 64)
		if err != nil || a >= o.size {
			s3err(w, 416, "InvalidRange")
			return
		}
		if hi != "" {
			b, _ = strconv.ParseInt(hi, 10, 64)
			if b >= o.size {
				b = o.size - 1
			}
		}
		status = 206
		h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", a, b, o.size))
	}
	h.Set("Content-Length", strconv.FormatInt(b-a+1, 10))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	f.gets.Add(1)
	if status == 206 {
		f.rangedGets.Add(1)
	}
	if o.data != nil {
		f.writeData(w, o.data[a:b+1], 0, fault)
	} else {
		f.writeData(w, nil, b-a+1, fault)
	}
}

func partEtag(id string, n int, size int64) string {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s/%d/%d", id, n, size)
	return fmt.Sprintf("%016x", h.Sum64())
}

func (f *memS3) putPart(w http.ResponseWriter, r *http.Request, path string, fault *memFault) {
	q := r.URL.Query()
	id := q.Get("uploadId")
	num, _ := strconv.Atoi(q.Get("partNumber"))
	f.mu.Lock()
	u := f.uploads[id]
	f.mu.Unlock()
	if u == nil {
		io.Copy(io.Discard, r.Body)
		s3err(w, 404, "NoSuchUpload")
		return
	}
	cur := f.curParts.Add(1)
	for {
		m := f.maxParts.Load()
		if cur <= m || f.maxParts.CompareAndSwap(m, cur) {
			break
		}
	}
	defer f.curParts.Add(-1)
	body, nb, ok := f.readBody(w, r, !f.synthetic, fault)
	if !ok {
		return
	}
	f.putBytes.Add(nb)
	f.partsPut.Add(1)
	p := &memPart{size: nb}
	if f.synthetic {
		p.etag = partEtag(id, num, nb)
	} else {
		sum := md5.Sum(body)
		p.etag = hex.EncodeToString(sum[:])
		p.data = body
	}
	f.mu.Lock()
	if _, ok := f.uploads[id]; !ok {
		f.mu.Unlock()
		s3err(w, 404, "NoSuchUpload")
		return
	}
	u.parts[num] = p
	f.mu.Unlock()
	w.Header().Set("ETag", `"`+p.etag+`"`)
}

func (f *memS3) complete(w http.ResponseWriter, r *http.Request, path string) {
	id := r.URL.Query().Get("uploadId")
	var req partsXML
	b, _ := io.ReadAll(r.Body)
	if err := xml.Unmarshal(b, &req); err != nil {
		s3err(w, 400, "MalformedXML")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	u := f.uploads[id]
	if u == nil {
		s3err(w, 404, "NoSuchUpload")
		return
	}
	var all []byte
	var sums []byte
	var total int64
	last := 0
	for _, cp := range req.Parts {
		if cp.PartNumber <= last {
			s3err(w, 400, "InvalidPartOrder")
			return
		}
		last = cp.PartNumber
		p := u.parts[cp.PartNumber]
		if p == nil || p.etag != strings.Trim(cp.ETag, `"`) {
			s3err(w, 400, "InvalidPart")
			return
		}
		total += p.size
		if !f.synthetic {
			all = append(all, p.data...)
			raw, _ := hex.DecodeString(p.etag)
			sums = append(sums, raw...)
		}
	}
	etag := "synth-" + strconv.Itoa(len(req.Parts))
	if !f.synthetic {
		s := md5.Sum(sums)
		etag = fmt.Sprintf("%s-%d", hex.EncodeToString(s[:]), len(req.Parts))
	}
	f.objs[u.key] = &memObj{data: all, size: total, etag: etag, header: u.header, modified: time.Now().UTC().Truncate(time.Second)}
	delete(f.uploads, id)
	w.Header().Set("Content-Type", "application/xml")
	bucket, key, _ := strings.Cut(u.key, "/")
	fmt.Fprintf(w, `<CompleteMultipartUploadResult><Location>x</Location><Bucket>%s</Bucket><Key>%s</Key><ETag>"%s"</ETag></CompleteMultipartUploadResult>`, bucket, key, etag)
}

func (f *memS3) listParts(w http.ResponseWriter, q map[string][]string, path string) {
	id := q["uploadId"][0]
	marker, maxParts := 0, 1000
	if v := q["part-number-marker"]; len(v) > 0 {
		marker, _ = strconv.Atoi(v[0])
	}
	if v := q["max-parts"]; len(v) > 0 {
		maxParts, _ = strconv.Atoi(v[0])
	}
	f.mu.Lock()
	u := f.uploads[id]
	if u == nil {
		f.mu.Unlock()
		s3err(w, 404, "NoSuchUpload")
		return
	}
	var nums []int
	for n := range u.parts {
		if n > marker {
			nums = append(nums, n)
		}
	}
	sort.Ints(nums)
	trunc := false
	if len(nums) > maxParts {
		nums, trunc = nums[:maxParts], true
	}
	var sb strings.Builder
	bucket, key, _ := strings.Cut(path, "/")
	next := 0
	for _, n := range nums {
		p := u.parts[n]
		next = n
		fmt.Fprintf(&sb, `<Part><PartNumber>%d</PartNumber><ETag>"%s"</ETag><Size>%d</Size><LastModified>%s</LastModified></Part>`, n, p.etag, p.size, time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<ListPartsResult><Bucket>%s</Bucket><Key>%s</Key><UploadId>%s</UploadId><PartNumberMarker>%d</PartNumberMarker><NextPartNumberMarker>%d</NextPartNumberMarker><MaxParts>%d</MaxParts><IsTruncated>%t</IsTruncated>%s</ListPartsResult>`, bucket, key, id, marker, next, maxParts, trunc, sb.String())
}

func (f *memS3) listUploads(w http.ResponseWriter, bucket, prefix string) {
	f.mu.Lock()
	var us []*memUpload
	for _, u := range f.uploads {
		b, k, _ := strings.Cut(u.key, "/")
		if b == bucket && strings.HasPrefix(k, prefix) {
			us = append(us, u)
		}
	}
	f.mu.Unlock()
	sort.Slice(us, func(i, j int) bool { return us[i].key < us[j].key || (us[i].key == us[j].key && us[i].id < us[j].id) })
	var sb strings.Builder
	for _, u := range us {
		_, k, _ := strings.Cut(u.key, "/")
		fmt.Fprintf(&sb, `<Upload><Key>%s</Key><UploadId>%s</UploadId><Initiator><ID>i</ID><DisplayName>i</DisplayName></Initiator><Owner><ID>o</ID><DisplayName>o</DisplayName></Owner><StorageClass>STANDARD</StorageClass><Initiated>%s</Initiated></Upload>`, k, u.id, u.initiated.UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<ListMultipartUploadsResult><Bucket>%s</Bucket><KeyMarker></KeyMarker><UploadIdMarker></UploadIdMarker><NextKeyMarker></NextKeyMarker><NextUploadIdMarker></NextUploadIdMarker><MaxUploads>1000</MaxUploads><IsTruncated>false</IsTruncated><Prefix>%s</Prefix>%s</ListMultipartUploadsResult>`, bucket, prefix, sb.String())
}

// patterned returns n deterministic non-repeating-looking bytes.
func patterned(n int, seed byte) []byte {
	b := make([]byte, n)
	x := uint32(seed)*2654435761 + 1
	for i := range b {
		x = x*1664525 + 1013904223
		b[i] = byte(x >> 24)
	}
	return b
}
