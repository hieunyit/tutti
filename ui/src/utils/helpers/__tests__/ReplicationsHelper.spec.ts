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
import { ReplicationsHelper } from '../ReplicationsHelper';
import type { TuttiReplication } from '@/utils/types/tuttiCore';

function repl(id: TuttiReplication['id']): TuttiReplication {
  return { id } as TuttiReplication;
}

describe('ReplicationsHelper.getSubjectLabel', () => {
  it('shows A → B for a storage-level replication with an empty user', () => {
    expect(
      ReplicationsHelper.getSubjectLabel(
        repl({ user: '', fromStorage: 'A', toStorage: 'B' }),
      ),
    ).toBe('A → B');
  });

  it('shows A → B when the user is omitted', () => {
    expect(
      ReplicationsHelper.getSubjectLabel(
        repl({ fromStorage: 'A', toStorage: 'B' }),
      ),
    ).toBe('A → B');
  });

  it('keeps the user for a real user', () => {
    expect(
      ReplicationsHelper.getSubjectLabel(
        repl({ user: 'alice', fromStorage: 'A', toStorage: 'B' }),
      ),
    ).toBe('alice');
  });

  it('prefers the bucket when there is one', () => {
    expect(
      ReplicationsHelper.getSubjectLabel(
        repl({
          user: '',
          fromStorage: 'A',
          toStorage: 'B',
          fromBucket: 'x',
          toBucket: 'x',
        }),
      ),
    ).toBe('x');
  });
});
