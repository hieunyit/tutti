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

import { I18nLocale, type I18nMessages } from '@clyso/clyso-ui-kit';

export default <I18nMessages>{
  [I18nLocale.EN]: {
    title: 'Add replication',
    loadError: 'Storages could not be loaded. Try again.',
    notEnoughStorages:
      'A replication needs two storages. Add another storage, then try again.',
    stepStorages: 'Storages',
    stepBuckets: 'Buckets',
    stepReview: 'Review',
    sourceLabel: 'Source storage',
    destinationLabel: 'Destination storage',
    bucketsSelected: 'Selected buckets',
    bucketsAll: 'All buckets on {storage}, including future ones',
    allDisabled: 'Not available: some buckets are {reason}.',
    searchBuckets: 'Search buckets',
    showReplicated: 'Show replicated buckets',
    selectedCount: '{total} selected',
    columnBucket: 'Bucket',
    columnStatus: 'Status',
    statusAvailable: 'Available',
    statusReplicated: 'Replicated',
    statusCovered: 'Covered',
    coveredByGroup: 'already covered by protection group {id}',
    coveredByReplication: 'already covered by replication {id}',
    noBuckets: 'No buckets on {storage}',
    noMatch: 'No buckets match',
    clearSearch: 'Clear search',
    retry: 'Try again',
    initBucketsError: 'The buckets could not be listed. Try again.',
    selectOne: 'Select at least one bucket to continue.',
    reviewSource: 'Source',
    reviewDestination: 'Destination',
    reviewBuckets: 'Buckets',
    reviewAll: 'All buckets, including future ones',
    reviewNote:
      'Existing objects are copied first, then changes are kept in sync.',
    create: 'Create replication',
    createdTitle: 'Replication created',
    createFailedTitle: 'The replication could not be created',
    createFailedHint: 'Your selection is kept. Change it or try again.',
    createFailedText: 'The server gave no reason.',
  },
  [I18nLocale.DE]: {
    title: 'Replikation hinzufügen',
    loadError:
      'Die Speicher konnten nicht geladen werden. Versuchen Sie es erneut.',
    notEnoughStorages:
      'Eine Replikation braucht zwei Speicher. Fügen Sie einen weiteren Speicher hinzu und versuchen Sie es erneut.',
    stepStorages: 'Speicher',
    stepBuckets: 'Buckets',
    stepReview: 'Prüfen',
    sourceLabel: 'Quellspeicher',
    destinationLabel: 'Zielspeicher',
    bucketsSelected: 'Ausgewählte Buckets',
    bucketsAll: 'Alle Buckets auf {storage}, auch künftige',
    allDisabled: 'Nicht verfügbar: Einige Buckets sind {reason}.',
    searchBuckets: 'Buckets suchen',
    showReplicated: 'Replizierte Buckets anzeigen',
    selectedCount: '{total} ausgewählt',
    columnBucket: 'Bucket',
    columnStatus: 'Status',
    statusAvailable: 'Verfügbar',
    statusReplicated: 'Repliziert',
    statusCovered: 'Abgedeckt',
    coveredByGroup: 'bereits von der Schutzgruppe {id} abgedeckt',
    coveredByReplication: 'bereits von der Replikation {id} abgedeckt',
    noBuckets: 'Keine Buckets auf {storage}',
    noMatch: 'Keine Buckets gefunden',
    clearSearch: 'Suche zurücksetzen',
    retry: 'Erneut versuchen',
    initBucketsError:
      'Die Buckets konnten nicht aufgelistet werden. Versuchen Sie es erneut.',
    selectOne: 'Wählen Sie mindestens einen Bucket, um fortzufahren.',
    reviewSource: 'Quelle',
    reviewDestination: 'Ziel',
    reviewBuckets: 'Buckets',
    reviewAll: 'Alle Buckets, auch künftige',
    reviewNote:
      'Vorhandene Objekte werden zuerst kopiert, danach werden Änderungen synchron gehalten.',
    create: 'Replikation erstellen',
    createdTitle: 'Replikation erstellt',
    createFailedTitle: 'Die Replikation konnte nicht erstellt werden',
    createFailedHint:
      'Ihre Auswahl bleibt erhalten. Ändern Sie sie oder versuchen Sie es erneut.',
    createFailedText: 'Der Server hat keinen Grund genannt.',
  },
  vi: {
    title: 'Thêm replication',
    loadError: 'Không tải được danh sách storage. Hãy thử lại.',
    notEnoughStorages:
      'Replication cần hai storage. Hãy thêm một storage nữa rồi thử lại.',
    stepStorages: 'Storage',
    stepBuckets: 'Bucket',
    stepReview: 'Xem lại',
    sourceLabel: 'Storage nguồn',
    destinationLabel: 'Storage đích',
    bucketsSelected: 'Các bucket đã chọn',
    bucketsAll: 'Tất cả bucket trên {storage}, kể cả bucket tạo sau này',
    allDisabled: 'Không khả dụng: một số bucket {reason}.',
    searchBuckets: 'Tìm bucket',
    showReplicated: 'Hiện bucket đã replicate',
    selectedCount: 'Đã chọn {total}',
    columnBucket: 'Bucket',
    columnStatus: 'Trạng thái',
    statusAvailable: 'Khả dụng',
    statusReplicated: 'Đã replicate',
    statusCovered: 'Đã được phủ',
    coveredByGroup: 'đã được phủ bởi nhóm bảo vệ {id}',
    coveredByReplication: 'đã được phủ bởi replication {id}',
    noBuckets: 'Không có bucket nào trên {storage}',
    noMatch: 'Không có bucket nào khớp',
    clearSearch: 'Xóa tìm kiếm',
    retry: 'Thử lại',
    initBucketsError: 'Không liệt kê được bucket. Hãy thử lại.',
    selectOne: 'Hãy chọn ít nhất một bucket để tiếp tục.',
    reviewSource: 'Nguồn',
    reviewDestination: 'Đích',
    reviewBuckets: 'Bucket',
    reviewAll: 'Tất cả bucket, kể cả bucket tạo sau này',
    reviewNote:
      'Các object hiện có được sao chép trước, sau đó các thay đổi được giữ đồng bộ.',
    create: 'Tạo replication',
    createdTitle: 'Đã tạo replication',
    createFailedTitle: 'Không thể tạo replication',
    createFailedHint:
      'Lựa chọn của bạn được giữ lại. Hãy đổi lựa chọn hoặc thử lại.',
    createFailedText: 'Server không nêu lý do.',
  },
};
