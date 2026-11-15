package store

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// MultipartResumeStore keeps the resume records of interrupted multipart uploads
// (key p:mpu:{<replication id>:<generation>}:<sha1(object key NUL version)>, JSON, TTL 7 days) in the queue
// Redis. The record format is owned by service/worker/copy; the store only moves bytes.
type MultipartResumeStore struct {
	client redis.Cmdable
}

func NewMultipartResumeStore(client redis.Cmdable) *MultipartResumeStore {
	return &MultipartResumeStore{client: client}
}

// Get returns (nil, nil) when the key does not exist.
func (s *MultipartResumeStore) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return b, err
}

func (s *MultipartResumeStore) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return s.client.Set(ctx, key, val, ttl).Err()
}

func (s *MultipartResumeStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
