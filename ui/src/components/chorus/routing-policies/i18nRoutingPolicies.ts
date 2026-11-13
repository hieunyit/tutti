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
    routingListTitle: 'Routing rules',
    pageDescription:
      'Decide which storage serves the requests of a user or bucket, and block access to a target when needed.',
    noResultsTitle: 'No routing policies yet',
    noResultsText:
      'Routing policies send the requests of a user or bucket to a specific storage.\nAdd one to get started.',
    filterNoResultsTitle: 'No routing policies found',
    filterNoResultsText:
      'There is no routing policy matching your filter criteria.',
    filterNoResultsAction: 'Clear filters',
    actionEdit: 'Change target storage',
    editTitle: 'Change target storage',
    editDescription: 'Select the new target storage for this routing policy.',
    editCurrent: 'Current storage',
    editNew: 'New target storage',
    editPlaceholder: 'Select storage',
    editNoStorages: 'No other storages are configured.',
    editSave: 'Save',
    editCancel: 'Cancel',
    editLoadError: 'The storage list could not be loaded.',
    editUnknownError: 'The routing policy could not be updated.',
    editSuccessTitle: 'Routing policy updated',
    routingPoliciesTitle: 'Routing policies',
    columnUser: 'User',
    columnBucket: 'Bucket',
    columnStorage: 'Storage',
    columnStatus: 'Status',
    statusAllowed: 'Allowed',
    statusBlocked: 'Blocked',
    routingPolicyStatusAllowed:
      'Access to the configured routing policy target allowed',
    routingPolicyStatusBlocked:
      'Access to the configured routing policy target blocked',
    routingPolicyStorage:
      'The storage the routing policy configuration is related to',
    allBuckets: 'All Buckets',
    allBucketsDescription: 'Apply to all buckets (including future ones)',
    filterByUserPlaceholder: 'Filter by User',
    filterByBucketPlaceholder: 'Filter by Bucket',
    filterByStoragePlaceholder: 'Filter by Storage',
    filterStatusAllowed: 'Allowed',
    filterStatusBlocked: 'Blocked',
    filterByStatusPlaceholder: 'Filter by Status',
    columnActions: 'Actions',
    actionDelete: 'Delete',
    actionDeleteBlocked: 'Unblock the routing policy before deleting it',
    actionBlock: 'Block',
    actionUnblock: 'Allow',
    routingPolicyDeletionConfirmTitle: 'Delete Routing Policy',
    routingPolicyDeletionConfirmContent:
      'You are about to delete the routing policy as follows:',
    routingPolicyDeletionConfirmQuestion: 'Are you sure you want to continue?',
    routingPolicyDeletionConfirmAction: 'Confirm Deletion',
    routingPolicyDeletionCancelAction: 'Cancel',
    deleteSuccessTitle: 'Deleted!',
    deleteSuccessContent: 'The following routing policies have been deleted:',
    deleteErrorTitle: 'Deletion failed!',
    deleteErrorContent:
      'An error occurred when deleting the following routing policies:',
    deleteErrorAction: 'Retry',
    deleteRoutingPolicyErrorUnknown: 'Unknown error',
    deleteErrorDetailsLabel: 'Details',
    cancel: 'Cancel',
    actionSelectedDeleteTitle: 'Delete Routing Policies',
    actionSelectedDeleteContent:
      'You are about to delete the following routing policies:',
    actionSelectedDeleteQuestion: 'Are you sure you want to continue?',
    deleteSelectedErrorContent:
      'An error occurred while deleting the following {total} routing policies:',
    deleteSelectedSuccessContent:
      'The following {total} routing policies have been deleted:',
    blockSuccessTitle: 'Blocked!',
    blockSuccessContent: 'The following routing policy has been blocked:',
    blockErrorTitle: 'Blocking failed!',
    blockErrorContent:
      'An error occurred while blocking the following routing policy:',
    blockErrorAction: 'Retry',
    unblockSuccessTitle: 'Allowed!',
    unblockSuccessContent: 'The following routing policy has been unblocked:',
    unblockErrorTitle: 'Unblocking failed!',
    unblockErrorContent:
      'An error occurred while unblocking the following routing policy:',
    unblockErrorAction: 'Retry',
    actionSelectedBlockTitle: 'Block Routing Policies',
    actionSelectedBlockContent:
      'You are about to block the following {total} routing policies:',
    actionSelectedBlockQuestion: 'Are you sure you want to continue?',
    blockSelectedSuccessContent:
      'The following {total} routing policies have been blocked:',
    blockSelectedErrorContent:
      'An error occurred while blocking the following {total} routing policies:',
    actionSelectedUnblockTitle: 'Allow Routing Policies',
    actionSelectedUnblockContent:
      'You are about to allow the following {total} routing policies:',
    actionSelectedUnblockQuestion: 'Are you sure you want to continue?',
    unblockSelectedSuccessContent:
      'The following {total} routing policies have been allowed:',
    unblockSelectedErrorContent:
      'An error occurred while allowing the following {total} routing policies:',
    actionAddRoutingPolicy: 'Add routing policy',
  },
  [I18nLocale.DE]: {
    routingListTitle: 'Routing-Regeln',
    pageDescription:
      'Legen Sie fest, welcher Speicher die Anfragen eines Benutzers oder Buckets bedient, und sperren Sie bei Bedarf den Zugriff auf ein Ziel.',
    noResultsTitle: 'Noch keine Routing-Richtlinien',
    noResultsText:
      'Routing-Richtlinien leiten die Anfragen eines Benutzers oder Buckets an einen bestimmten Speicher.\nFügen Sie eine hinzu, um zu beginnen.',
    filterNoResultsTitle: 'Keine Routing-Richtlinien gefunden',
    filterNoResultsText:
      'Keine Routing-Richtlinie entspricht Ihren Filterkriterien.',
    filterNoResultsAction: 'Filter zurücksetzen',
    actionEdit: 'Zielspeicher ändern',
    editTitle: 'Zielspeicher ändern',
    editDescription:
      'Wählen Sie den neuen Zielspeicher für diese Routing-Richtlinie.',
    editCurrent: 'Aktueller Speicher',
    editNew: 'Neuer Zielspeicher',
    editPlaceholder: 'Speicher auswählen',
    editNoStorages: 'Es sind keine weiteren Speicher konfiguriert.',
    editSave: 'Speichern',
    editCancel: 'Abbrechen',
    editLoadError: 'Die Speicherliste konnte nicht geladen werden.',
    editUnknownError:
      'Die Routing-Richtlinie konnte nicht aktualisiert werden.',
    editSuccessTitle: 'Routing-Richtlinie aktualisiert',
    routingPoliciesTitle: 'Routing-Richtlinien',
    columnUser: 'Benutzer',
    columnBucket: 'Bucket',
    columnStorage: 'Speicher',
    columnStatus: 'Status',
    statusAllowed: 'Zugelassen',
    statusBlocked: 'Blockiert',
    routingPolicyStatusAllowed:
      'Zugriff auf das konfigurierte Ziel der Routing-Richtlinie erlaubt',
    routingPolicyStatusBlocked:
      'Zugriff auf das konfigurierte Ziel der Routing-Richtlinie blockiert',
    routingPolicyStorage:
      'Der Storage, auf den sich die Routing-Richtlinie bezieht',
    allBuckets: 'Alle Buckets',
    allBucketsDescription:
      'Auf alle Buckets anwenden (einschließlich zukünftiger)',
    filterByUserPlaceholder: 'Nach Benutzer filtern',
    filterByBucketPlaceholder: 'Nach Bucket filtern',
    filterByStoragePlaceholder: 'Nach Speicherort filtern',
    filterByStorageNoResult: 'Kein Speicher entspricht Ihren Suchkriterien',
    filterStatusAllowed: 'Zugelassen',
    filterStatusBlocked: 'Blockiert',
    filterByStatusPlaceholder: 'Nach Status filtern',
    columnActions: 'Aktionen',
    actionDelete: 'Löschen',
    actionDeleteBlocked: 'Routing-Richtlinie vor dem Löschen entsperren',
    actionBlock: 'Blockieren',
    actionUnblock: 'Entsperren',
    routingPolicyDeletionConfirmTitle: 'Routing-Richtlinie Löschen',
    routingPolicyDeletionConfirmContent:
      'Sie löschen folgende Routing-Richtlinie:',
    routingPolicyDeletionConfirmQuestion: 'Möchten Sie fortfahren?',
    routingPolicyDeletionConfirmAction: 'Löschen bestätigen',
    routingPolicyDeletionCancelAction: 'Abbrechen',
    deleteSuccessTitle: 'Gelöscht!',
    deleteSuccessContent: 'Die folgenden Routing-Richtlinien wurde gelöscht:',
    deleteErrorTitle: 'Löschen fehlgeschlagen!',
    deleteErrorContent:
      'Beim Löschen der folgenden Routing-Richtlinien ist ein Fehler aufgetreten:',
    deleteErrorAction: 'Erneut versuchen',
    deleteRoutingPolicyErrorUnknown: 'Unbekannter Fehler',
    deleteErrorDetailsLabel: 'Details',
    cancel: 'Abbrechen',
    actionSelectedDeleteTitle: 'Routing-Richtlinien Löschen',
    actionSelectedDeleteContent: 'Sie löschen folgende Routing-Richtlinien:',
    actionSelectedDeleteQuestion: 'Möchten Sie fortfahren?',
    deleteSelectedErrorContent:
      'Es ist ein Fehler beim Löschen der folgenden {total} Routing-Richtlinien aufgetreten:',
    deleteSelectedSuccessContent:
      'Die folgenden {total} Routing-Richtlinien wurden gelöscht:',
    blockSuccessTitle: 'Blockiert!',
    blockSuccessContent: 'Die folgende Routing-Richtlinie wurde blockiert:',
    blockErrorTitle: 'Blockieren fehlgeschlagen!',
    blockErrorContent:
      'Beim Blockieren der folgenden Routing-Richtlinie ist ein Fehler aufgetreten:',
    blockErrorAction: 'Erneut versuchen',
    unblockSuccessTitle: 'Entsperrt!',
    unblockSuccessContent: 'Die folgende Routing-Richtlinie wurde entsperrt:',
    unblockErrorTitle: 'Entsperren fehlgeschlagen!',
    unblockErrorContent:
      'Beim Entsperren der folgenden Routing-Richtlinie ist ein Fehler aufgetreten:',
    unblockErrorAction: 'Erneut versuchen',
    actionSelectedBlockTitle: 'Routing-Richtlinien Blockieren',
    actionSelectedBlockContent: 'Sie blockieren folgende Routing-Richtlinien:',
    actionSelectedBlockQuestion: 'Möchten Sie fortfahren?',
    blockSelectedSuccessContent:
      'Die folgenden {total} Routing-Richtlinien wurden blockiert:',
    blockSelectedErrorContent:
      'Es ist ein Fehler beim Blockieren der folgenden {total} Routing-Richtlinien aufgetreten:',
    actionSelectedUnblockTitle: 'Routing-Richtlinien Entsperren',
    actionSelectedUnblockContent:
      'Sie entsperren folgende Routing-Richtlinien:',
    actionSelectedUnblockQuestion: 'Möchten Sie fortfahren?',
    unblockSelectedSuccessContent:
      'Die folgenden {total} Routing-Richtlinien wurden entsperrt:',
    unblockSelectedErrorContent:
      'Es ist ein Fehler beim Entsperren der folgenden {total} Routing-Richtlinien aufgetreten:',
    actionAddRoutingPolicy: 'Routing-Richtlinie Hinzufügen',
  },
  vi: {
    routingListTitle: 'Quy tắc định tuyến',
    pageDescription:
      'Quyết định storage nào phục vụ request của một user hoặc bucket, và chặn truy cập tới một đích khi cần.',
    noResultsTitle: 'Chưa có routing policy',
    noResultsText:
      'Routing policy gửi request của một user hoặc bucket tới một storage cụ thể.\nHãy thêm một policy để bắt đầu.',
    filterNoResultsTitle: 'Không tìm thấy routing policy',
    filterNoResultsText: 'Không có routing policy nào khớp với tiêu chí lọc.',
    filterNoResultsAction: 'Xóa bộ lọc',
    actionEdit: 'Đổi storage đích',
    editTitle: 'Đổi storage đích',
    editDescription: 'Chọn storage đích mới cho routing policy này.',
    editCurrent: 'Storage hiện tại',
    editNew: 'Storage đích mới',
    editPlaceholder: 'Chọn storage',
    editNoStorages: 'Chưa cấu hình storage nào khác.',
    editSave: 'Lưu',
    editCancel: 'Hủy',
    editLoadError: 'Không tải được danh sách storage.',
    editUnknownError: 'Không thể cập nhật routing policy.',
    editSuccessTitle: 'Đã cập nhật routing policy',
    routingPoliciesTitle: 'Routing policy',
    columnUser: 'User',
    columnBucket: 'Bucket',
    columnStorage: 'Storage',
    columnStatus: 'Trạng thái',
    statusAllowed: 'Cho phép',
    statusBlocked: 'Bị chặn',
    routingPolicyStatusAllowed:
      'Cho phép truy cập tới đích của routing policy đã cấu hình',
    routingPolicyStatusBlocked:
      'Chặn truy cập tới đích của routing policy đã cấu hình',
    routingPolicyStorage: 'Storage mà cấu hình routing policy liên quan tới',
    allBuckets: 'Tất cả bucket',
    allBucketsDescription:
      'Áp dụng cho tất cả bucket (kể cả bucket tạo sau này)',
    filterByUserPlaceholder: 'Lọc theo user',
    filterByBucketPlaceholder: 'Lọc theo bucket',
    filterByStoragePlaceholder: 'Lọc theo storage',
    filterStatusAllowed: 'Cho phép',
    filterStatusBlocked: 'Bị chặn',
    filterByStatusPlaceholder: 'Lọc theo trạng thái',
    columnActions: 'Thao tác',
    actionDelete: 'Xóa',
    actionDeleteBlocked: 'Hãy bỏ chặn routing policy trước khi xóa',
    actionBlock: 'Chặn',
    actionUnblock: 'Cho phép',
    routingPolicyDeletionConfirmTitle: 'Xóa routing policy',
    routingPolicyDeletionConfirmContent: 'Bạn sắp xóa routing policy sau:',
    routingPolicyDeletionConfirmQuestion: 'Bạn có chắc muốn tiếp tục?',
    routingPolicyDeletionConfirmAction: 'Xác nhận xóa',
    routingPolicyDeletionCancelAction: 'Hủy',
    deleteSuccessTitle: 'Đã xóa!',
    deleteSuccessContent: 'Đã xóa các routing policy sau:',
    deleteErrorTitle: 'Xóa thất bại!',
    deleteErrorContent: 'Đã xảy ra lỗi khi xóa các routing policy sau:',
    deleteErrorAction: 'Thử lại',
    deleteRoutingPolicyErrorUnknown: 'Lỗi không xác định',
    deleteErrorDetailsLabel: 'Chi tiết',
    cancel: 'Hủy',
    actionSelectedDeleteTitle: 'Xóa routing policy',
    actionSelectedDeleteContent: 'Bạn sắp xóa các routing policy sau:',
    actionSelectedDeleteQuestion: 'Bạn có chắc muốn tiếp tục?',
    deleteSelectedErrorContent:
      'Đã xảy ra lỗi khi xóa {total} routing policy sau:',
    deleteSelectedSuccessContent: 'Đã xóa {total} routing policy sau:',
    blockSuccessTitle: 'Đã chặn!',
    blockSuccessContent: 'Đã chặn routing policy sau:',
    blockErrorTitle: 'Chặn thất bại!',
    blockErrorContent: 'Đã xảy ra lỗi khi chặn routing policy sau:',
    blockErrorAction: 'Thử lại',
    unblockSuccessTitle: 'Đã cho phép!',
    unblockSuccessContent: 'Đã bỏ chặn routing policy sau:',
    unblockErrorTitle: 'Bỏ chặn thất bại!',
    unblockErrorContent: 'Đã xảy ra lỗi khi bỏ chặn routing policy sau:',
    unblockErrorAction: 'Thử lại',
    actionSelectedBlockTitle: 'Chặn routing policy',
    actionSelectedBlockContent: 'Bạn sắp chặn {total} routing policy sau:',
    actionSelectedBlockQuestion: 'Bạn có chắc muốn tiếp tục?',
    blockSelectedSuccessContent: 'Đã chặn {total} routing policy sau:',
    blockSelectedErrorContent:
      'Đã xảy ra lỗi khi chặn {total} routing policy sau:',
    actionSelectedUnblockTitle: 'Cho phép routing policy',
    actionSelectedUnblockContent:
      'Bạn sắp cho phép {total} routing policy sau:',
    actionSelectedUnblockQuestion: 'Bạn có chắc muốn tiếp tục?',
    unblockSelectedSuccessContent: 'Đã cho phép {total} routing policy sau:',
    unblockSelectedErrorContent:
      'Đã xảy ra lỗi khi cho phép {total} routing policy sau:',
    actionAddRoutingPolicy: 'Thêm routing policy',
  },
};
