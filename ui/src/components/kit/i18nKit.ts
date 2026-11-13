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

// Strings of the shared kit components, keyed by locale code (en, de, vi).
// Plain codes instead of the UI kit's I18nLocale keep the kit free of the
// @clyso/clyso-ui-kit bundle, so kit components mount in unit tests as is.
export default <Record<string, Record<string, string>>>{
  en: {
    loading: 'Loading',
    retry: 'Try again',
    errorTitle: 'Could not load this page',
    errorText: 'The server did not answer. Check the connection and try again.',
    emptyTitle: 'Nothing here yet',
    search: 'Search',
    clearFilters: 'Clear filters',
    removeFilter: 'Remove filter {label}',
    cancel: 'Cancel',
    affects: 'This affects:',
    typeToConfirm: 'Type {name} to confirm',
    selected: '{n} selected',
    clearSelection: 'Clear selection',
    next: 'Next',
    back: 'Back',
    finish: 'Finish',
    startOver: 'Start over',
    stepOf: 'Step {current} of {total}',
    copy: 'Copy',
    copied: 'Copied',
    copyFailed: 'Copy failed',
    oneTimeWarning:
      'This value is shown only once. Copy it and store it somewhere safe now.',
    savedConfirm: 'I have saved this value',
    trendUp: 'up',
    trendDown: 'down',
    trendFlat: 'unchanged',
  },
  de: {
    loading: 'Wird geladen',
    retry: 'Erneut versuchen',
    errorTitle: 'Diese Seite konnte nicht geladen werden',
    errorText:
      'Der Server hat nicht geantwortet. Prüfen Sie die Verbindung und versuchen Sie es erneut.',
    emptyTitle: 'Noch keine Einträge',
    search: 'Suchen',
    clearFilters: 'Filter zurücksetzen',
    removeFilter: 'Filter {label} entfernen',
    cancel: 'Abbrechen',
    affects: 'Betroffen:',
    typeToConfirm: 'Geben Sie {name} zur Bestätigung ein',
    selected: '{n} ausgewählt',
    clearSelection: 'Auswahl aufheben',
    next: 'Weiter',
    back: 'Zurück',
    finish: 'Abschließen',
    startOver: 'Neu beginnen',
    stepOf: 'Schritt {current} von {total}',
    copy: 'Kopieren',
    copied: 'Kopiert',
    copyFailed: 'Kopieren fehlgeschlagen',
    oneTimeWarning:
      'Dieser Wert wird nur einmal angezeigt. Kopieren Sie ihn jetzt und bewahren Sie ihn sicher auf.',
    savedConfirm: 'Ich habe diesen Wert gespeichert',
    trendUp: 'gestiegen',
    trendDown: 'gesunken',
    trendFlat: 'unverändert',
  },
  vi: {
    loading: 'Đang tải',
    retry: 'Thử lại',
    errorTitle: 'Không tải được trang này',
    errorText: 'Máy chủ không phản hồi. Kiểm tra kết nối rồi thử lại.',
    emptyTitle: 'Chưa có dữ liệu',
    search: 'Tìm kiếm',
    clearFilters: 'Xóa lọc',
    removeFilter: 'Bỏ lọc {label}',
    cancel: 'Hủy',
    affects: 'Đối tượng bị ảnh hưởng:',
    typeToConfirm: 'Gõ {name} để xác nhận',
    selected: 'Đã chọn {n}',
    clearSelection: 'Bỏ chọn',
    next: 'Tiếp',
    back: 'Quay lại',
    finish: 'Hoàn tất',
    startOver: 'Làm lại từ đầu',
    stepOf: 'Bước {current}/{total}',
    copy: 'Sao chép',
    copied: 'Đã sao chép',
    copyFailed: 'Không sao chép được',
    oneTimeWarning:
      'Giá trị này chỉ hiện một lần. Hãy sao chép và lưu ở nơi an toàn ngay bây giờ.',
    savedConfirm: 'Tôi đã lưu giá trị này',
    trendUp: 'tăng',
    trendDown: 'giảm',
    trendFlat: 'không đổi',
  },
};
