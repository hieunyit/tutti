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

package storagetest

import (
	"context"
	"slices"

	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/objstore"
)

// HealthProbe returns the availability probe of an S3 storage for the health checker (pkg/health):
//   - with a connection account: steps 1-5 of the connection test (DNS+TCP, TLS, S3 endpoint, clock skew,
//     authentication). A failure is reported with the step name, e.g. "authentication: SignatureDoesNotMatch".
//   - otherwise (legacy): ListBuckets with the credentials of the first user that has its own credentials.
//
// Because the connection account is read on every round, a rotation is picked up without restarting the probe.
func HealthProbe(clients objstore.Clients, creds objstore.CredsService, storage string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if ci, ok := creds.ConnectionAccount(storage); ok {
			addr, err := creds.GetS3Address(storage)
			if err != nil {
				return err
			}
			return Probe(ctx, Options{Address: addr, Cred: ci.Credentials})
		}
		users := creds.ListOverrideUsers(storage)
		if len(users) == 0 {
			return health.ErrSkipProbe
		}
		slices.Sort(users)
		c, err := clients.AsS3(ctx, storage, users[0])
		if err != nil {
			return err
		}
		_, err = c.S3().ListBuckets(ctx)
		return err
	}
}
