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

import type { StatusTone } from '@/components/kit';
import { ReplicationsHelper } from '@/utils/helpers/ReplicationsHelper';
import type { TuttiReplication } from '@/utils/types/tuttiCore';

export interface ReplicationStatus {
  tone: StatusTone;
  label: string;
  /** live events not yet replicated (0 while the initial copy runs) */
  behind: number;
}

type Translate = (key: string) => string;

/** One status per replication: paused beats initial copy beats lag. */
export function getReplicationStatus(
  replication: TuttiReplication,
  t: Translate,
): ReplicationStatus {
  const { isPaused, isInitDone } = replication;
  const behind = isInitDone
    ? Math.max(
        0,
        ReplicationsHelper.getLiveReplicationEventsDifference(replication),
      )
    : 0;

  if (isPaused) return { tone: 'warning', label: t('statusPaused'), behind };

  if (!isInitDone)
    return { tone: 'info', label: t('statusInitial'), behind: 0 };

  return behind > 0
    ? { tone: 'warning', label: t('statusLagging'), behind }
    : { tone: 'success', label: t('statusUptodate'), behind };
}
