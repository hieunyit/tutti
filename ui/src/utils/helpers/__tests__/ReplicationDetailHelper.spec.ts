/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import { describe, expect, it } from 'vitest';
import { ReplicationDetailHelper as H } from '../ReplicationDetailHelper';

describe('ReplicationDetailHelper', () => {
  it('round-trips a bucket replication through the query', () => {
    const id = {
      user: 'alice',
      fromStorage: 'a',
      toStorage: 'b',
      fromBucket: 'x',
      toBucket: 'y',
    };

    expect(H.fromQuery(H.toQuery(id) as never)).toEqual(id);
  });

  it('omits empty fields and rejects a query without storages', () => {
    expect(H.toQuery({ fromStorage: 'a', toStorage: 'b' })).toEqual({
      from: 'a',
      to: 'b',
    });
    expect(H.fromQuery({ from: 'a' })).toBeNull();
    expect(H.fromQuery({})).toBeNull();
    expect(H.fromQuery({ from: ['a', 'z'], to: 'b' })).toEqual({
      fromStorage: 'a',
      toStorage: 'b',
    });
  });
});
