// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package proxy

import (
	"context"
	"sync"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/storagetest"
)

type changingStorages interface {
	objstore.CredsService
	Subscribe(func([]objstore.StorageChange))
}

// Subscribe before starting probes: a definition change is applied immediately on
// the credentials poll, without waiting for the old storage's probe interval.
func manageStorageHealth(ctx context.Context, creds changingStorages, clients objstore.Clients, reg *health.Registry, onRegister func(string)) {
	var mu sync.Mutex
	probes := map[string]context.CancelFunc{}
	reconcile := func(changes []objstore.StorageChange) {
		mu.Lock()
		defer mu.Unlock()
		for _, change := range changes {
			if cancel := probes[change.Name]; cancel != nil {
				cancel()
				delete(probes, change.Name)
			}
			if change.Kind == "removed" {
				reg.Remove(change.Name)
				continue
			}
			addr, err := creds.GetS3Address(change.Name)
			if err != nil {
				continue
			}
			reg.Reset(change.Name)
			if onRegister != nil {
				onRegister(change.Name)
			}
			probeCtx, cancel := context.WithCancel(ctx)
			probes[change.Name] = cancel
			go reg.RunChecks(probeCtx, health.Target{Storage: change.Name, Interval: addr.HealthCheckInterval, Probe: storagetest.HealthProbe(clients, creds, change.Name)})
		}
	}
	creds.Subscribe(reconcile)
	var initial []objstore.StorageChange
	for name, kind := range creds.Storages() {
		if kind == dom.S3 {
			initial = append(initial, objstore.StorageChange{Name: name, Kind: "added"})
		}
	}
	reconcile(initial)
}
