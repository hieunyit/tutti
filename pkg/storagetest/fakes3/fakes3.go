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

// Package fakes3 is a small in-memory S3 server for tests: it checks the access key of the Authorization header (not
// the signature), can deny single operations and answers the calls of the connection test.
package fakes3

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Request is one recorded request.
type Request struct {
	Op        string
	Bucket    string
	Key       string
	AccessKey string
}

type version struct {
	id     string
	data   []byte
	marker bool
}

type bucket struct {
	versioned bool
	objects   map[string][]version
	uploads   map[string]string
}

// Server is the fake storage.
type Server struct {
	*httptest.Server

	mu sync.Mutex
	// AccessKeys are the access keys that authenticate. Default: {"tutti-svc"}.
	AccessKeys map[string]bool
	// Deny lists operations answered with AccessDenied (names as in the connection test: PutObject, DeleteBucket, ...).
	Deny map[string]bool
	// DenyBuckets lists buckets on which every operation is denied.
	DenyBuckets map[string]bool
	// AuthError, if set, is the error code of every authenticated request (e.g. SignatureDoesNotMatch).
	AuthError string
	// DateOffset is added to the Date header.
	DateOffset time.Duration
	// NoVersioning answers the versioning calls with NotImplemented.
	NoVersioning bool
	// VersionPageSize bounds pages in ListObjectVersions for pagination tests.
	VersionPageSize int
	// Owner is the owner display name of ListBuckets.
	Owner string
	// Region is the bucket location.
	Region string
	// Requests is the log of requests.
	Requests []Request

	buckets map[string]*bucket
}

var uploadRe = regexp.MustCompile(`^[0-9a-f]+$`)

// New starts a plain HTTP fake.
func New() *Server { return wrap(httptest.NewUnstartedServer(nil)).start(false) }

// NewTLS starts a fake with a self-signed certificate.
func NewTLS() *Server { return wrap(httptest.NewUnstartedServer(nil)).start(true) }

func wrap(h *httptest.Server) *Server {
	return &Server{Server: h, AccessKeys: map[string]bool{"tutti-svc": true}, Deny: map[string]bool{}, DenyBuckets: map[string]bool{},
		Owner: "tutti", Region: "us-east-1", buckets: map[string]*bucket{}}
}

func (s *Server) start(tls bool) *Server {
	s.Server.Config.Handler = http.HandlerFunc(s.handle)
	if tls {
		s.Server.StartTLS()
	} else {
		s.Server.Start()
	}
	return s
}

// Buckets returns the bucket names.
func (s *Server) Buckets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for b := range s.buckets {
		out = append(out, b)
	}
	sort.Strings(out)
	return out
}

// CreateBucket adds a bucket.
func (s *Server) CreateBucket(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buckets[name] = &bucket{objects: map[string][]version{}, uploads: map[string]string{}}
}

// SeedVersionedObject creates an existing versioned object for connection-test fixtures.
func (s *Server) SeedVersionedObject(bucketName, key string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.buckets[bucketName]
	if b == nil {
		b = &bucket{objects: map[string][]version{}, uploads: map[string]string{}}
		s.buckets[bucketName] = b
	}
	b.versioned = true
	b.objects[key] = []version{{id: "original", data: append([]byte(nil), data...)}}
}

// Objects returns the number of stored objects (all versions) of the bucket.
func (s *Server) Objects(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.buckets[name]
	if b == nil {
		return 0
	}
	n := 0
	for _, v := range b.objects {
		n += len(v)
	}
	return n
}

// Seen returns the recorded access keys, unique and sorted.
func (s *Server) Seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	set := map[string]bool{}
	for _, r := range s.Requests {
		if r.AccessKey != "" {
			set[r.AccessKey] = true
		}
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Ops returns the recorded operation names in order.
func (s *Server) Ops() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.Requests))
	for _, r := range s.Requests {
		out = append(out, r.Op)
	}
	return out
}

func xmlErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>%s</Code><Message>%s</Message></Error>`, code, msg)
}

func accessKey(r *http.Request) string {
	a := r.Header.Get("Authorization")
	_, rest, ok := strings.Cut(a, "Credential=")
	if !ok {
		return ""
	}
	ak, _, _ := strings.Cut(rest, "/")
	return ak
}

func op(r *http.Request) (name, bkt, key string) {
	p := strings.TrimPrefix(r.URL.Path, "/")
	bkt, key, _ = strings.Cut(p, "/")
	q := r.URL.Query()
	has := func(k string) bool { _, ok := q[k]; return ok }
	m := r.Method
	switch {
	case bkt == "":
		return "ListBuckets", "", ""
	case key == "":
		switch {
		case has("location"):
			return "GetBucketLocation", bkt, ""
		case has("versioning"):
			return map[string]string{"GET": "GetBucketVersioning", "PUT": "PutBucketVersioning"}[m], bkt, ""
		case has("versions"):
			return "ListObjectVersions", bkt, ""
		case has("uploads"):
			return "ListMultipartUploads", bkt, ""
		case has("tagging"):
			return map[string]string{"GET": "GetBucketTagging", "PUT": "PutBucketTagging", "DELETE": "DeleteBucketTagging"}[m], bkt, ""
		case has("acl"):
			return map[string]string{"GET": "GetBucketAcl", "PUT": "PutBucketAcl"}[m], bkt, ""
		case has("lifecycle"):
			return map[string]string{"GET": "GetBucketLifecycleConfiguration", "PUT": "PutBucketLifecycleConfiguration", "DELETE": "DeleteBucketLifecycle"}[m], bkt, ""
		case has("policy"):
			return map[string]string{"GET": "GetBucketPolicy", "PUT": "PutBucketPolicy", "DELETE": "DeleteBucketPolicy"}[m], bkt, ""
		}
		return map[string]string{"PUT": "CreateBucket", "DELETE": "DeleteBucket", "HEAD": "HeadBucket", "GET": "ListObjectsV2"}[m], bkt, ""
	}
	switch {
	case has("uploads") && m == "POST":
		return "CreateMultipartUpload", bkt, key
	case has("uploadId"):
		return map[string]string{"PUT": "UploadPart", "GET": "ListParts", "DELETE": "AbortMultipartUpload"}[m], bkt, key
	case has("tagging"):
		return map[string]string{"GET": "GetObjectTagging", "PUT": "PutObjectTagging"}[m], bkt, key
	case has("acl"):
		return map[string]string{"GET": "GetObjectAcl", "PUT": "PutObjectAcl"}[m], bkt, key
	case has("versionId") && m == "DELETE":
		return "DeleteObjectVersion", bkt, key
	}
	return map[string]string{"PUT": "PutObject", "GET": "GetObject", "HEAD": "HeadObject", "DELETE": "DeleteObject"}[m], bkt, key
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Date", time.Now().Add(s.DateOffset).UTC().Format(http.TimeFormat))
	body, _ := io.ReadAll(r.Body)
	name, bkt, key := op(r)
	ak := accessKey(r)
	s.Requests = append(s.Requests, Request{Op: name, Bucket: bkt, Key: key, AccessKey: ak})
	if ak == "" {
		xmlErr(w, 403, "AccessDenied", "Access Denied.")
		return
	}
	if !s.AccessKeys[ak] {
		xmlErr(w, 403, "InvalidAccessKeyId", "The Access Key Id you provided does not exist in our records.")
		return
	}
	if s.AuthError != "" {
		xmlErr(w, 403, s.AuthError, "fake authentication failure")
		return
	}
	if s.Deny[name] || (bkt != "" && s.DenyBuckets[bkt]) {
		xmlErr(w, 403, "AccessDenied", "Access Denied.")
		return
	}
	q := r.URL.Query()
	if strings.Contains(name, "Versioning") || name == "ListObjectVersions" || name == "DeleteObjectVersion" {
		if s.NoVersioning {
			xmlErr(w, 501, "NotImplemented", "A header you provided implies functionality that is not implemented")
			return
		}
	}
	if name == "ListBuckets" {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><ListAllMyBucketsResult><Owner><ID>0123</ID><DisplayName>%s</DisplayName></Owner><Buckets>`, s.Owner)
		names := make([]string, 0, len(s.buckets))
		for b := range s.buckets {
			names = append(names, b)
		}
		sort.Strings(names)
		for _, b := range names {
			fmt.Fprintf(w, "<Bucket><Name>%s</Name></Bucket>", b)
		}
		fmt.Fprint(w, "</Buckets></ListAllMyBucketsResult>")
		return
	}
	b := s.buckets[bkt]
	if name == "CreateBucket" {
		if b != nil {
			xmlErr(w, 409, "BucketAlreadyOwnedByYou", "exists")
			return
		}
		s.buckets[bkt] = &bucket{objects: map[string][]version{}, uploads: map[string]string{}}
		return
	}
	if b == nil {
		if name == "HeadBucket" {
			w.WriteHeader(404)
			return
		}
		xmlErr(w, 404, "NoSuchBucket", "The specified bucket does not exist")
		return
	}
	ok := func() { w.Header().Set("Content-Type", "application/xml") }
	switch name {
	case "HeadBucket":
	case "DeleteBucket":
		if len(b.objects) > 0 || len(b.uploads) > 0 {
			xmlErr(w, 409, "BucketNotEmpty", "The bucket you tried to delete is not empty")
			return
		}
		delete(s.buckets, bkt)
		w.WriteHeader(204)
	case "GetBucketLocation":
		ok()
		fmt.Fprintf(w, `<LocationConstraint xmlns="http://s3.amazonaws.com/doc/2006-03-01/">%s</LocationConstraint>`, s.Region)
	case "GetBucketVersioning":
		ok()
		st := ""
		if b.versioned {
			st = "<Status>Enabled</Status>"
		}
		fmt.Fprintf(w, `<VersioningConfiguration>%s</VersioningConfiguration>`, st)
	case "PutBucketVersioning":
		b.versioned = strings.Contains(string(body), "Enabled")
	case "ListObjectsV2":
		ok()
		prefix := q.Get("prefix")
		keys := make([]string, 0)
		for k, vs := range b.objects {
			if strings.HasPrefix(k, prefix) && len(vs) > 0 && !vs[len(vs)-1].marker {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		if mk := q.Get("max-keys"); mk == "1" && len(keys) > 1 {
			keys = keys[:1]
		}
		fmt.Fprint(w, `<ListBucketResult>`)
		for _, k := range keys {
			fmt.Fprintf(w, "<Contents><Key>%s</Key></Contents>", k)
		}
		fmt.Fprintf(w, "<KeyCount>%d</KeyCount></ListBucketResult>", len(keys))
	case "ListObjectVersions":
		ok()
		fmt.Fprint(w, `<ListVersionsResult>`)
		keys := make([]string, 0)
		for k := range b.objects {
			if strings.HasPrefix(k, q.Get("prefix")) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		type listed struct {
			key string
			v   version
		}
		var all []listed
		for _, k := range keys {
			for _, v := range b.objects[k] {
				all = append(all, listed{key: k, v: v})
			}
		}
		if marker := q.Get("key-marker"); marker != "" {
			for i, item := range all {
				if item.key == marker && item.v.id == q.Get("version-id-marker") {
					all = all[i+1:]
					break
				}
			}
		}
		if s.VersionPageSize > 0 && len(all) > s.VersionPageSize {
			all = all[:s.VersionPageSize]
			last := all[len(all)-1]
			fmt.Fprintf(w, "<IsTruncated>true</IsTruncated><NextKeyMarker>%s</NextKeyMarker><NextVersionIdMarker>%s</NextVersionIdMarker>", last.key, last.v.id)
		}
		for _, item := range all {
			tag := "Version"
			if item.v.marker {
				tag = "DeleteMarker"
			}
			fmt.Fprintf(w, "<%s><Key>%s</Key><VersionId>%s</VersionId></%s>", tag, item.key, item.v.id, tag)
		}
		fmt.Fprint(w, `</ListVersionsResult>`)
	case "ListMultipartUploads":
		ok()
		fmt.Fprint(w, `<ListMultipartUploadsResult>`)
		for id, k := range b.uploads {
			fmt.Fprintf(w, "<Upload><Key>%s</Key><UploadId>%s</UploadId></Upload>", k, id)
		}
		fmt.Fprint(w, `</ListMultipartUploadsResult>`)
	case "GetBucketTagging", "GetBucketAcl", "GetBucketLifecycleConfiguration", "GetBucketPolicy", "PutBucketTagging", "PutBucketAcl",
		"PutBucketLifecycleConfiguration", "PutBucketPolicy", "DeleteBucketPolicy", "DeleteBucketTagging", "DeleteBucketLifecycle":
		if strings.HasPrefix(name, "Delete") {
			w.WriteHeader(204)
		}
	case "CreateMultipartUpload":
		id := make([]byte, 6)
		_, _ = rand.Read(id)
		uid := hex.EncodeToString(id)
		b.uploads[uid] = key
		ok()
		fmt.Fprintf(w, `<InitiateMultipartUploadResult><UploadId>%s</UploadId></InitiateMultipartUploadResult>`, uid)
	case "UploadPart":
		if _, found := b.uploads[q.Get("uploadId")]; !found {
			xmlErr(w, 404, "NoSuchUpload", "no such upload")
			return
		}
		w.Header().Set("ETag", `"part"`)
	case "ListParts":
		if _, found := b.uploads[q.Get("uploadId")]; !found {
			xmlErr(w, 404, "NoSuchUpload", "no such upload")
			return
		}
		ok()
		fmt.Fprint(w, `<ListPartsResult><Part><PartNumber>1</PartNumber></Part></ListPartsResult>`)
	case "AbortMultipartUpload":
		delete(b.uploads, q.Get("uploadId"))
		w.WriteHeader(204)
	case "PutObject":
		vs := b.objects[key]
		v := version{data: body}
		if b.versioned {
			id := make([]byte, 4)
			_, _ = rand.Read(id)
			v.id = hex.EncodeToString(id)
			w.Header().Set("X-Amz-Version-Id", v.id)
			b.objects[key] = append(vs, v)
		} else {
			b.objects[key] = []version{v}
		}
	case "GetObject", "HeadObject":
		vs := b.objects[key]
		if len(vs) == 0 || vs[len(vs)-1].marker {
			xmlErr(w, 404, "NoSuchKey", "The specified key does not exist.")
			return
		}
		if name == "GetObject" {
			_, _ = w.Write(vs[len(vs)-1].data)
		}
	case "GetObjectTagging", "PutObjectTagging", "GetObjectAcl", "PutObjectAcl":
	case "DeleteObject":
		if b.versioned {
			id := make([]byte, 4)
			_, _ = rand.Read(id)
			markerID := hex.EncodeToString(id)
			b.objects[key] = append(b.objects[key], version{id: markerID, marker: true})
			w.Header().Set("X-Amz-Version-Id", markerID)
			w.Header().Set("X-Amz-Delete-Marker", "true")
		} else {
			delete(b.objects, key)
		}
		w.WriteHeader(204)
	case "DeleteObjectVersion":
		id := q.Get("versionId")
		var keep []version
		for _, v := range b.objects[key] {
			if v.id != id {
				keep = append(keep, v)
			}
		}
		if len(keep) == 0 {
			delete(b.objects, key)
		} else {
			b.objects[key] = keep
		}
		w.WriteHeader(204)
	default:
		xmlErr(w, 501, "NotImplemented", "unsupported fake operation "+name)
	}
	_ = uploadRe
}
