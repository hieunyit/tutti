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

import { I18nLocale, type I18nMessages } from '@clyso/clyso-ui-kit';

export default <I18nMessages>{
  [I18nLocale.EN]: {
    backToDiffReports: 'Diff reports',
    errorTitle: 'Error',
    errorText:
      'An error occurred while loading the diff report.\nPlease try again.',
    errorAction: 'Retry',
    notFoundText: 'The requested diff report could not be found.',
    // Header
    detailTitle: 'Diff report',
    // Overview
    statusChecking: 'Checking',
    statusConsistent: 'Consistent',
    statusInconsistent: 'Inconsistent',
    configVersioned: 'Versioned',
    configNotVersioned: 'Not versioned',
    configConsidersEtags: 'ETags compared',
    configIgnoresEtags: 'ETags ignored',
    configConsidersSizes: 'Sizes compared',
    configIgnoresSizes: 'Sizes ignored',
    // Progress
    fixProgressTitle: 'Fix progress',
    progressLabel: 'Progress',
    progressInconsistentObjects: 'Inconsistent objects',
    // Restart
    // Entries
    entriesTitle: 'Differences',
    entriesSearchPlaceholder: 'Search by object name...',
    columnObject: 'Object',
    columnSize: 'Size',
    columnEtag: 'ETag',
    columnObjectPlacement: 'Object Placement',
    versionIdx: 'version idx {idx}',
    progressCount: '{done} of {total} objects checked',
    overviewComparison: 'Compared by',
  },
  [I18nLocale.DE]: {
    backToDiffReports: 'Diff-Reports',
    errorTitle: 'Fehler',
    errorText:
      'Beim Laden des Diff-Reports ist ein Fehler aufgetreten.\nBitte versuchen Sie es erneut.',
    errorAction: 'Erneut versuchen',
    notFoundText: 'Der angeforderte Diff-Report wurde nicht gefunden.',
    // Header
    detailTitle: 'Diff-Report',
    // Overview
    statusChecking: 'Prüfung läuft',
    statusConsistent: 'Konsistent',
    statusInconsistent: 'Inkonsistent',
    configVersioned: 'Versioniert',
    configNotVersioned: 'Nicht versioniert',
    configConsidersEtags: 'ETags verglichen',
    configIgnoresEtags: 'ETags ignoriert',
    configConsidersSizes: 'Größen verglichen',
    configIgnoresSizes: 'Größen ignoriert',
    // Progress
    fixProgressTitle: 'Fix-Fortschritt',
    progressLabel: 'Fortschritt',
    progressInconsistentObjects: 'Inkonsistente Objekte',
    // Restart
    // Entries
    entriesTitle: 'Unterschiede',
    entriesSearchPlaceholder: 'Nach Objektname suchen...',
    columnObject: 'Objekt',
    columnSize: 'Größe',
    columnEtag: 'ETag',
    columnObjectPlacement: 'Objekt-Platzierung',
    versionIdx: 'Version-Index {idx}',
    progressCount: '{done} von {total} Objekten geprüft',
    overviewComparison: 'Verglichen nach',
  },
  vi: {
    backToDiffReports: 'Báo cáo diff',
    errorTitle: 'Lỗi',
    errorText: 'Đã xảy ra lỗi khi tải báo cáo diff.\nVui lòng thử lại.',
    errorAction: 'Thử lại',
    notFoundText: 'Không tìm thấy báo cáo diff được yêu cầu.',
    detailTitle: 'Báo cáo diff',
    statusChecking: 'Đang kiểm tra',
    statusConsistent: 'Nhất quán',
    statusInconsistent: 'Không nhất quán',
    configVersioned: 'Có versioning',
    configNotVersioned: 'Không có versioning',
    configConsidersEtags: 'So sánh ETag',
    configIgnoresEtags: 'Bỏ qua ETag',
    configConsidersSizes: 'So sánh kích thước',
    configIgnoresSizes: 'Bỏ qua kích thước',
    fixProgressTitle: 'Tiến độ sửa',
    progressLabel: 'Tiến độ',
    progressInconsistentObjects: 'Object không nhất quán',
    entriesTitle: 'Khác biệt',
    entriesSearchPlaceholder: 'Tìm theo tên object...',
    columnObject: 'Object',
    columnSize: 'Kích thước',
    columnEtag: 'ETag',
    columnObjectPlacement: 'Vị trí object',
    versionIdx: 'version idx {idx}',
    progressCount: 'Đã kiểm tra {done}/{total} object',
    overviewComparison: 'So sánh theo',
  },
};
