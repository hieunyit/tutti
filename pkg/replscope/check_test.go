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

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func TestCheck(t *testing.T) {
	existing := []Owner{
		{Kind: "group", ID: "g1", Scope: GroupScope([]string{"A", "B", "C"}, nil)},
		{Kind: "replication", ID: "r1", Scope: ReplicationScope("X", "Y", []string{"photos"})},
	}
	require.Nil(t, Check(ReplicationScope("A", "D", []string{"x"}), existing, ""))

	err := Check(ReplicationScope("C", "A", nil), existing, "")
	require.NotNil(t, err)
	require.Equal(t, "group", err.OwnerKind)
	require.Equal(t, "g1", err.OwnerID)
	require.Equal(t, "*", err.Bucket)

	err = Check(ReplicationScope("Z", "Y", []string{"photos"}), existing, "")
	require.NotNil(t, err)
	require.Equal(t, "r1", err.OwnerID)
	require.Contains(t, err.Error(), `bucket "photos" on Z->Y is already covered by replication "r1"`)
	require.True(t, errors.Is(err, dom.ErrAlreadyExists))

	require.Nil(t, Check(ReplicationScope("Z", "Y", []string{"photos"}), existing, "r1"))
	require.Nil(t, Check(ReplicationScope("Z", "Y", []string{"other"}), existing, ""))
}
