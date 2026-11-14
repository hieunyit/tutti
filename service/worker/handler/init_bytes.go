package handler

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
)

// Initial replication byte counters are best effort: a Redis hiccup must never fail or retry an object copy.

func (s *svc) initBytes() *store.InitBytesStore {
	if rc := s.redisClient(); rc != nil {
		return store.NewInitBytesStore(rc)
	}
	return nil
}

// addInitTotalBytes registers sizes of objects accepted by the broker as copy tasks.
func (s *svc) addInitTotalBytes(ctx context.Context, id entity.UniversalReplicationID, generation int64, sizes []int64) {
	var total int64
	for _, n := range sizes {
		total += n
	}
	st := s.initBytes()
	if st == nil || total <= 0 {
		return
	}
	if err := st.AddTotal(ctx, id.AsString(), generation, total); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to update initial replication total bytes")
	}
}

// addInitDoneBytes registers an object whose initial copy task completed successfully.
func (s *svc) addInitDoneBytes(ctx context.Context, id entity.UniversalReplicationID, generation, size int64) {
	st := s.initBytes()
	if st == nil || size <= 0 {
		return
	}
	if err := st.AddDone(ctx, id.AsString(), generation, size, time.Now()); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to update initial replication done bytes")
	}
}
