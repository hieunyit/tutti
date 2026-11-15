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
    add: 'Add routing rule',
    search: 'Search routing rules',
    searchPlaceholder: 'User, bucket or storage',
    clearFilters: 'Clear filters',
    emptyTitle: 'No routing rules yet',
    emptyText:
      'A routing rule sends the requests of a user or bucket to one storage.',
    emptyFiltered: 'No routing rules match',
    loadError: 'The routing rules could not be loaded. Try again.',
    colUser: 'User',
    colBucket: 'Bucket',
    colStorage: 'Storage',
    colStatus: 'Status',
    allBuckets: 'All buckets',
    allBucketsHint: 'Applies to all buckets, including future ones',
    noStorage: 'None (requests are blocked)',
    statusAllowed: 'Allowed',
    statusBlocked: 'Blocked',
    moreActions: 'More actions',
    block: 'Block',
    unblock: 'Allow',
    changeStorage: 'Change storage',
    delete: 'Delete routing rule',
    deleteMany: 'Delete {n} routing rules',
    deleteText:
      'Requests of this user and bucket are routed by the next matching rule or the default route.',
    deleteManyText: 'Requests fall back to the next matching rule.',
    deleteBlocked: 'Allow the routing rule before deleting it',
    actionDone: 'Done',
    actionFailed: 'Some changes could not be applied. Try again.',
    unknownError: 'The change could not be applied.',
    editTitle: 'Change storage',
    editCurrent: 'Current storage',
    editNew: 'New storage',
    editNoStorages: 'No other storage is available.',
    editSave: 'Save',
    editCancel: 'Cancel',
    editLoadError:
      'The storage list could not be loaded. Close this dialog and try again.',
    editUnknownError: 'The routing rule could not be updated. Try again.',
    editSuccess: 'Routing rule updated',
  },
  [I18nLocale.DE]: {
    add: 'Routing-Regel hinzufügen',
    search: 'Routing-Regeln durchsuchen',
    searchPlaceholder: 'Benutzer, Bucket oder Speicher',
    clearFilters: 'Filter zurücksetzen',
    emptyTitle: 'Noch keine Routing-Regeln',
    emptyText:
      'Eine Routing-Regel leitet die Anfragen eines Benutzers oder Buckets an einen Speicher.',
    emptyFiltered: 'Keine Routing-Regeln gefunden',
    loadError:
      'Die Routing-Regeln konnten nicht geladen werden. Versuchen Sie es erneut.',
    colUser: 'Benutzer',
    colBucket: 'Bucket',
    colStorage: 'Speicher',
    colStatus: 'Status',
    allBuckets: 'Alle Buckets',
    allBucketsHint: 'Gilt für alle Buckets, auch für künftige',
    noStorage: 'Keiner (Anfragen werden blockiert)',
    statusAllowed: 'Zugelassen',
    statusBlocked: 'Blockiert',
    moreActions: 'Weitere Aktionen',
    block: 'Blockieren',
    unblock: 'Zulassen',
    changeStorage: 'Speicher ändern',
    delete: 'Routing-Regel löschen',
    deleteMany: '{n} Routing-Regeln löschen',
    deleteText:
      'Anfragen dieses Benutzers und Buckets werden von der nächsten passenden Regel oder der Standardroute geleitet.',
    deleteManyText: 'Anfragen fallen auf die nächste passende Regel zurück.',
    deleteBlocked: 'Lassen Sie die Routing-Regel zu, bevor Sie sie löschen',
    actionDone: 'Erledigt',
    actionFailed:
      'Einige Änderungen konnten nicht angewendet werden. Versuchen Sie es erneut.',
    unknownError: 'Die Änderung konnte nicht angewendet werden.',
    editTitle: 'Speicher ändern',
    editCurrent: 'Aktueller Speicher',
    editNew: 'Neuer Speicher',
    editNoStorages: 'Es ist kein weiterer Speicher verfügbar.',
    editSave: 'Speichern',
    editCancel: 'Abbrechen',
    editLoadError:
      'Die Speicherliste konnte nicht geladen werden. Schließen Sie den Dialog und versuchen Sie es erneut.',
    editUnknownError:
      'Die Routing-Regel konnte nicht aktualisiert werden. Versuchen Sie es erneut.',
    editSuccess: 'Routing-Regel aktualisiert',
  },
  vi: {
    add: 'Thêm routing rule',
    search: 'Tìm routing rule',
    searchPlaceholder: 'User, bucket hoặc storage',
    clearFilters: 'Xóa bộ lọc',
    emptyTitle: 'Chưa có routing rule',
    emptyText:
      'Routing rule gửi request của một user hoặc bucket tới một storage.',
    emptyFiltered: 'Không có routing rule nào khớp',
    loadError: 'Không tải được routing rule. Hãy thử lại.',
    colUser: 'User',
    colBucket: 'Bucket',
    colStorage: 'Storage',
    colStatus: 'Trạng thái',
    allBuckets: 'Tất cả bucket',
    allBucketsHint: 'Áp dụng cho mọi bucket, kể cả bucket tạo sau này',
    noStorage: 'Không có (request bị chặn)',
    statusAllowed: 'Cho phép',
    statusBlocked: 'Bị chặn',
    moreActions: 'Thao tác khác',
    block: 'Chặn',
    unblock: 'Cho phép',
    changeStorage: 'Đổi storage',
    delete: 'Xóa routing rule',
    deleteMany: 'Xóa {n} routing rule',
    deleteText:
      'Request của user và bucket này sẽ theo routing rule khớp tiếp theo hoặc route mặc định.',
    deleteManyText: 'Request sẽ theo routing rule khớp tiếp theo.',
    deleteBlocked: 'Hãy cho phép routing rule trước khi xóa',
    actionDone: 'Xong',
    actionFailed: 'Một số thay đổi chưa được áp dụng. Hãy thử lại.',
    unknownError: 'Không thể áp dụng thay đổi.',
    editTitle: 'Đổi storage',
    editCurrent: 'Storage hiện tại',
    editNew: 'Storage mới',
    editNoStorages: 'Không còn storage nào khác.',
    editSave: 'Lưu',
    editCancel: 'Hủy',
    editLoadError:
      'Không tải được danh sách storage. Hãy đóng hộp thoại và thử lại.',
    editUnknownError: 'Không thể cập nhật routing rule. Hãy thử lại.',
    editSuccess: 'Đã cập nhật routing rule',
  },
};
