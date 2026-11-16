/*
 * Copyright © 2026 Clyso GmbH
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

import {
  type TuttiCoveredBucket,
  type TuttiBucketListRequest,
  type TuttiStorage,
} from '@/utils/types/tuttiCore';
import {
  ReplicationSinceHelper,
  type SinceChoice,
} from '@/utils/helpers/ReplicationSinceHelper';

export interface TuttiAddReplicationState {
  isLoading: boolean;
  hasError: boolean;
  storages: TuttiStorage[];
  selectedFromStorage: TuttiStorage | null;
  selectedToStorage: TuttiStorage | null;
  isForAllBuckets: boolean;
  isBucketsListLoading: boolean;
  hasBucketsError: boolean;
  bucketsError: string;
  isReplicatedShown: boolean;
  bucketsRequestOptions: TuttiBucketListRequest | null;
  bucketsList: string[];
  replicatedBucketsList: string[];
  coveredBucketsList: TuttiCoveredBucket[];
  /** server text of the last failed create, shown on the page */
  createError: string;
  selectedBuckets: string[];
  /** optional display name applied to every replication created */
  name: string;
  /** which objects to copy: all, the last N days or hours, or since a date */
  since: SinceChoice;
  /** rescan the source every N minutes; 0 = off */
  rescanMinutes: number;
  isSubmitting: boolean;
}

export function getInitialState(): TuttiAddReplicationState {
  return {
    isLoading: false,
    hasError: false,
    storages: [],
    selectedFromStorage: null,
    selectedToStorage: null,
    isForAllBuckets: false,
    isBucketsListLoading: false,
    hasBucketsError: false,
    bucketsError: '',
    isReplicatedShown: false,
    bucketsRequestOptions: null,
    bucketsList: [],
    replicatedBucketsList: [],
    coveredBucketsList: [],
    createError: '',
    selectedBuckets: [],
    name: '',
    since: ReplicationSinceHelper.defaultChoice(),
    rescanMinutes: 0,
    isSubmitting: false,
  };
}
