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
    pageDescription:
      'Compare the contents of buckets across storages and review the differences between them.',
    noResultsAction: 'New diff report',
    filterNoResultsTitle: 'No diff reports found',
    filterNoResultsText:
      'There is no diff report matching your filter criteria.',
    filterNoResultsAction: 'Clear filters',
    columnStatus: 'Status',
    columnActions: 'Actions',
    statusChecking: 'Checking',
    statusConsistent: 'Consistent',
    statusInconsistent: 'Inconsistent',
    configVersioned: 'Versioned',
    configNotVersioned: 'Not versioned',
    configIgnoresEtags: 'Ignores ETags',
    configIgnoresSizes: 'Ignores Sizes',
    filterStatusChecking: 'Checking',
    filterStatusConsistent: 'Consistent',
    filterStatusInconsistent: 'Inconsistent',
    noResultsTitle: 'No Diff Reports yet',
    noResultsText: 'There are no diff reports available at the moment.',
    errorTitle: 'Error',
    errorText:
      'An error occurred while getting the diff report list.\nPlease try one more time.',
    errorAction: 'Retry',

    restartDiffReportConfirmContent:
      'You are about to restart the diff report generation for the following diff report:',

    filterAny: 'Any',
    filterLabelStorage: 'Storage',
    filterLabelBucket: 'Bucket',
    filterLabelStatus: 'Status',
    moreActions: 'More actions',
    cliReport: 'Created with the CLI: use the CLI for details.',
    checkedPercent: 'Checked {n}%',
  },
  [I18nLocale.DE]: {
    pageDescription:
      'Vergleichen Sie die Inhalte von Buckets über Speicher hinweg und prüfen Sie die Unterschiede.',
    noResultsAction: 'Diff-Report erstellen',
    filterNoResultsTitle: 'Keine Diff-Reports gefunden',
    filterNoResultsText: 'Kein Diff-Report entspricht Ihren Filterkriterien.',
    filterNoResultsAction: 'Filter zurücksetzen',
    columnStatus: 'Status',
    columnActions: 'Aktionen',
    statusChecking: 'Prüfung läuft',
    statusConsistent: 'Konsistent',
    statusInconsistent: 'Inkonsistent',
    configVersioned: 'Versioniert',
    configNotVersioned: 'Nicht versioniert',
    configIgnoresEtags: 'Ignoriert ETags',
    configIgnoresSizes: 'Ignoriert Dateigrößen',
    filterStatusChecking: 'Prüfung läuft',
    filterStatusConsistent: 'Konsistent',
    filterStatusInconsistent: 'Inkonsistent',
    noResultsTitle: 'Noch keine Diff-Reports',
    noResultsText: 'Derzeit sind keine Diff-Reports verfügbar.',
    errorTitle: 'Fehler',
    errorText:
      'Beim Abrufen der Diff-Report-Liste ist ein Fehler aufgetreten.\nBitte versuchen Sie es erneut.',
    errorAction: 'Erneut versuchen',

    restartDiffReportConfirmContent:
      'Sie sind dabei, die Diff-Report-Generierung für den folgenden Diff-Report neu zu starten:',

    filterAny: 'Alle',
    filterLabelStorage: 'Speicher',
    filterLabelBucket: 'Bucket',
    filterLabelStatus: 'Status',
    moreActions: 'Weitere Aktionen',
    cliReport: 'Über die CLI erstellt: Details in der CLI.',
    checkedPercent: '{n} % geprüft',
  },
  vi: {
    pageDescription:
      'So sánh nội dung bucket giữa các storage và xem lại những khác biệt giữa chúng.',
    noResultsAction: 'Báo cáo diff mới',
    filterNoResultsTitle: 'Không tìm thấy báo cáo diff',
    filterNoResultsText: 'Không có báo cáo diff nào khớp với tiêu chí lọc.',
    filterNoResultsAction: 'Xóa bộ lọc',
    columnStatus: 'Trạng thái',
    columnActions: 'Thao tác',
    statusChecking: 'Đang kiểm tra',
    statusConsistent: 'Nhất quán',
    statusInconsistent: 'Không nhất quán',
    configVersioned: 'Versioned',
    configNotVersioned: 'Không versioned',
    configIgnoresEtags: 'Bỏ qua ETag',
    configIgnoresSizes: 'Bỏ qua kích thước',
    filterStatusChecking: 'Đang kiểm tra',
    filterStatusConsistent: 'Nhất quán',
    filterStatusInconsistent: 'Không nhất quán',
    noResultsTitle: 'Chưa có báo cáo diff',
    noResultsText: 'Hiện chưa có báo cáo diff nào.',
    errorTitle: 'Lỗi',
    errorText:
      'Đã xảy ra lỗi khi lấy danh sách báo cáo diff.\nVui lòng thử lại một lần nữa.',
    errorAction: 'Thử lại',
    restartDiffReportConfirmContent:
      'Bạn sắp chạy lại việc tạo báo cáo diff cho báo cáo diff sau:',
    filterAny: 'Tất cả',
    filterLabelStorage: 'Storage',
    filterLabelBucket: 'Bucket',
    filterLabelStatus: 'Trạng thái',
    moreActions: 'Thao tác khác',
    cliReport: 'Tạo bằng CLI: xem chi tiết trong CLI.',
    checkedPercent: 'Đã kiểm tra {n}%',
  },
};
