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
    notFoundTitle: 'Report Not Found',
    notFoundText: 'The requested diff report could not be found.',
    invalidLocationsTitle: 'Invalid Locations',
    invalidLocationsText: 'Diff reports require exactly two locations.',
    invalidLocationsAction: 'Back to Diff Reports',
    notFoundAction: 'Back to Diff Reports',
    // Header
    detailTitle: 'Diff report',
    detailSubtitle:
      'Diff report comparing objects between the source and destination locations.',
    // Overview
    overviewTitle: 'Report Overview',
    overviewStatus: 'Status',
    overviewConsistency: 'Consistency',
    overviewVersioned: 'Versioned',
    overviewEtags: 'ETags',
    overviewSizes: 'Sizes',
    statusReady: 'Ready',
    statusChecking: 'Checking',
    statusConsistent: 'Consistent',
    statusInconsistent: 'Inconsistent',
    configVersioned: 'Versioned',
    configNotVersioned: 'Not versioned',
    configConsidersEtags: 'Considers ETags',
    configIgnoresEtags: 'Ignores ETags',
    configConsidersSizes: 'Considers Sizes',
    configIgnoresSizes: 'Ignores Sizes',
    // Progress
    diffProgressTitle: 'Diff Report progress',
    fixProgressTitle: 'Fix progress',
    progressLabel: 'Progress',
    progressQueued: 'Queued',
    progressCompleted: 'Completed',
    progressInconsistentObjects: 'Inconsistent objects',
    fixReady: 'Fix ready',
    fixQueued: 'Fix queued',
    fixCompleted: 'Fix completed',
    fixReadyYes: 'Yes',
    fixReadyNo: 'No',
    // Restart
    restartAction: 'Restart',
    restartConfirmTitle: 'Restart Diff Report',
    restartConfirmContent:
      'You are about to restart this diff report. Are you sure you want to proceed?',
    restartConfirmAction: 'Restart',
    restartCancelAction: 'Cancel',
    restartSuccessTitle: 'Restarted!',
    restartSuccessContent: 'Diff report restarted.',
    restartErrorTitle: 'Restart failed!',
    restartErrorContent:
      'An error occurred while restarting the diff report. You may see outdated report information.',
    errorDetailsLabel: 'Error details: ',
    restartErrorAction: 'Retry',
    // Entries
    entriesTitle: 'Inconsistent Objects',
    entriesSearchPlaceholder: 'Search by object name...',
    columnObject: 'Object',
    columnSize: 'Size',
    columnEtag: 'ETag',
    columnObjectPlacement: 'Object Placement',
    versionIdx: 'version idx {idx}',
  },
  [I18nLocale.DE]: {
    backToDiffReports: 'Diff-Reports',
    errorTitle: 'Fehler',
    errorText:
      'Beim Laden des Diff-Reports ist ein Fehler aufgetreten.\nBitte versuchen Sie es erneut.',
    errorAction: 'Erneut versuchen',
    notFoundTitle: 'Report nicht gefunden',
    notFoundText: 'Der angeforderte Diff-Report wurde nicht gefunden.',
    invalidLocationsTitle: 'Ungültige Speicherorte',
    invalidLocationsText: 'Diff-Reports erfordern genau zwei Speicherorte.',
    invalidLocationsAction: 'Zurück zu Diff-Reports',
    notFoundAction: 'Zurück zu Diff-Reports',
    // Header
    detailTitle: 'Diff-Report',
    detailSubtitle:
      'Diff-Report zum Vergleich von Objekten zwischen Quell- und Ziel-Standorten.',
    // Overview
    overviewTitle: 'Report-Übersicht',
    overviewStatus: 'Status',
    overviewConsistency: 'Konsistenz',
    overviewVersioned: 'Versioniert',
    overviewEtags: 'ETags',
    overviewSizes: 'Dateigrößen',
    statusReady: 'Bereit',
    statusChecking: 'Prüfung läuft',
    statusConsistent: 'Konsistent',
    statusInconsistent: 'Inkonsistent',
    configVersioned: 'Versioniert',
    configNotVersioned: 'Nicht versioniert',
    configConsidersEtags: 'Berücksichtigt ETags',
    configIgnoresEtags: 'Ignoriert ETags',
    configConsidersSizes: 'Berücksichtigt Dateigrößen',
    configIgnoresSizes: 'Ignoriert Dateigrößen',
    // Progress
    diffProgressTitle: 'Diff-Report Fortschritt',
    fixProgressTitle: 'Fix-Fortschritt',
    progressLabel: 'Fortschritt',
    progressQueued: 'In Warteschlange',
    progressCompleted: 'Abgeschlossen',
    progressInconsistentObjects: 'Inkonsistente Objekte',
    fixReady: 'Fix bereit',
    fixQueued: 'Fix in Warteschlange',
    fixCompleted: 'Fix abgeschlossen',
    fixReadyYes: 'Ja',
    fixReadyNo: 'Nein',
    // Restart
    restartAction: 'Neustarten',
    restartConfirmTitle: 'Diff-Report neustarten',
    restartConfirmContent:
      'Sie sind dabei, diesen Diff-Report neu zu starten. Sind Sie sicher, dass Sie fortfahren möchten?',
    restartConfirmAction: 'Neustarten',
    restartCancelAction: 'Abbrechen',
    restartSuccessTitle: 'Neugestartet!',
    restartSuccessContent: 'Diff-Report neugestartet.',
    restartErrorTitle: 'Neustart fehlgeschlagen!',
    restartErrorContent:
      'Beim Neustart des Diff-Reports ist ein Fehler aufgetreten. Die angezeigten Report Details könnten möglicherweise nicht aktuell sein.',
    errorDetailsLabel: 'Error Details: ',
    restartErrorAction: 'Neustarten',
    // Entries
    entriesTitle: 'Inkonsistente Objekte',
    entriesSearchPlaceholder: 'Nach Objektname suchen...',
    columnObject: 'Objekt',
    columnSize: 'Größe',
    columnEtag: 'ETag',
    columnObjectPlacement: 'Objekt-Platzierung',
    versionIdx: 'Version-Index {idx}',
  },
  vi: {
    backToDiffReports: 'Báo cáo diff',
    errorTitle: 'Lỗi',
    errorText: 'Đã xảy ra lỗi khi tải báo cáo diff.\nVui lòng thử lại.',
    errorAction: 'Thử lại',
    notFoundTitle: 'Không tìm thấy báo cáo',
    notFoundText: 'Không tìm thấy báo cáo diff được yêu cầu.',
    invalidLocationsTitle: 'Vị trí không hợp lệ',
    invalidLocationsText: 'Báo cáo diff yêu cầu đúng hai vị trí.',
    invalidLocationsAction: 'Quay lại báo cáo diff',
    notFoundAction: 'Quay lại báo cáo diff',
    detailTitle: 'Báo cáo diff',
    detailSubtitle:
      'Báo cáo diff so sánh các object giữa vị trí nguồn và đích.',
    overviewTitle: 'Tổng quan báo cáo',
    overviewStatus: 'Trạng thái',
    overviewConsistency: 'Tính nhất quán',
    overviewVersioned: 'Versioned',
    overviewEtags: 'ETag',
    overviewSizes: 'Kích thước',
    statusReady: 'Sẵn sàng',
    statusChecking: 'Đang kiểm tra',
    statusConsistent: 'Nhất quán',
    statusInconsistent: 'Không nhất quán',
    configVersioned: 'Versioned',
    configNotVersioned: 'Không versioned',
    configConsidersEtags: 'Xét ETag',
    configIgnoresEtags: 'Bỏ qua ETag',
    configConsidersSizes: 'Xét kích thước',
    configIgnoresSizes: 'Bỏ qua kích thước',
    diffProgressTitle: 'Tiến độ báo cáo diff',
    fixProgressTitle: 'Tiến độ sửa',
    progressLabel: 'Tiến độ',
    progressQueued: 'Trong hàng đợi',
    progressCompleted: 'Hoàn tất',
    progressInconsistentObjects: 'Object không nhất quán',
    fixReady: 'Sẵn sàng sửa',
    fixQueued: 'Đã xếp hàng sửa',
    fixCompleted: 'Đã sửa xong',
    fixReadyYes: 'Có',
    fixReadyNo: 'Không',
    restartAction: 'Chạy lại',
    restartConfirmTitle: 'Chạy lại báo cáo diff',
    restartConfirmContent:
      'Bạn sắp chạy lại báo cáo diff này. Bạn có chắc muốn tiếp tục?',
    restartConfirmAction: 'Chạy lại',
    restartCancelAction: 'Hủy',
    restartSuccessTitle: 'Đã chạy lại!',
    restartSuccessContent: 'Đã chạy lại báo cáo diff.',
    restartErrorTitle: 'Chạy lại thất bại!',
    restartErrorContent:
      'Đã xảy ra lỗi khi chạy lại báo cáo diff. Thông tin báo cáo có thể đã cũ.',
    errorDetailsLabel: 'Chi tiết lỗi: ',
    restartErrorAction: 'Thử lại',
    entriesTitle: 'Object không nhất quán',
    entriesSearchPlaceholder: 'Tìm theo tên object...',
    columnObject: 'Object',
    columnSize: 'Kích thước',
    columnEtag: 'ETag',
    columnObjectPlacement: 'Vị trí object',
    versionIdx: 'version idx {idx}',
  },
};
