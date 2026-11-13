// Copyright 2025 Clyso GmbH
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

package replscope

// Check returns the first conflict between proposed and existing owners, or nil when allowed.
// The owner whose ID equals ignoreID is skipped (used when updating an object).
func Check(proposed Scope, existing []Owner, ignoreID string) *ConflictError {
	for _, o := range existing {
		if ignoreID != "" && o.ID == ignoreID {
			continue
		}
		if ok, bucket, e, other := Conflicts(proposed, o.Scope); ok {
			return &ConflictError{OwnerKind: o.Kind, OwnerID: o.ID, Bucket: bucket, Edge: e, OtherEdge: other}
		}
	}
	return nil
}
