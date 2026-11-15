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
import { getReplicationStatus } from '../replicationStatus';
import type { TuttiReplication } from '@/utils/types/tuttiCore';

const t = (key: string) => key;
const make = (patch: Partial<TuttiReplication>) =>
  ({
    isPaused: false,
    isInitDone: true,
    events: '10',
    eventsDone: '10',
    ...patch,
  }) as TuttiReplication;

describe('getReplicationStatus', () => {
  it('puts paused before everything else', () => {
    const status = getReplicationStatus(
      make({ isPaused: true, isInitDone: false }),
      t,
    );

    expect(status).toMatchObject({ tone: 'warning', label: 'statusPaused' });
  });

  it('reports the initial copy and no backlog while it runs', () => {
    const status = getReplicationStatus(
      make({ isInitDone: false, events: '50', eventsDone: '1' }),
      t,
    );

    expect(status).toMatchObject({
      tone: 'info',
      label: 'statusInitial',
      behind: 0,
    });
  });

  it('reports lag only when events are behind', () => {
    expect(getReplicationStatus(make({ events: '15' }), t)).toMatchObject({
      tone: 'warning',
      label: 'statusLagging',
      behind: 5,
    });
    expect(getReplicationStatus(make({}), t)).toMatchObject({
      tone: 'success',
      label: 'statusUptodate',
      behind: 0,
    });
  });
});
