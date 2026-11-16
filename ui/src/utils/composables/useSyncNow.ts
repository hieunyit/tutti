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

import { useNotification } from '@clyso/clyso-ui-kit';
import { useI18n } from 'vue-i18n';
import i18nReplicationRescan from '@/components/tutti/replications/i18nReplicationRescan';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import {
  ReplicationRescanHelper as H,
  type RescanBlock,
} from '@/utils/helpers/ReplicationRescanHelper';
import type {
  TuttiReplication,
  TuttiReplicationId,
} from '@/utils/types/tuttiCore';

/**
 * "Sync now" for the list: starts a rescan and reports the result in a toast.
 * Also gives the reason text of a disabled action.
 */
export function useSyncNow() {
  const { t } = useI18n({ messages: i18nReplicationRescan });
  const { createNotification } = useNotification();

  const blockKeys: Record<RescanBlock, string> = {
    paused: 'syncBlockedPaused',
    initial: 'syncBlockedInitial',
    running: 'syncBlockedRunning',
  };

  function blockedReason(replication: TuttiReplication): string {
    const block = H.blockedBy(replication);

    return block ? t(blockKeys[block]) : '';
  }

  async function syncNow(id: TuttiReplicationId) {
    try {
      await TuttiCoreService.rescanReplication(id);
      createNotification({
        type: 'success',
        title: t('syncStarted'),
        duration: 4000,
      });
    } catch (error) {
      createNotification({
        type: 'error',
        title: t('syncFailed'),
        content: H.errorText(error, ''),
        isClosable: true,
      });
    }
  }

  return { syncNow, blockedReason, syncLabel: () => t('syncNow') };
}
