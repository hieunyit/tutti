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
    backToRoutingPolicies: 'Routing policies',
    breadcrumbAddRoutingPolicy: 'Add Routing Policy',
    addRoutingPolicyTitle: 'Add routing policy',
    addRoutingPolicyHeader:
      'Create a new routing policy to control traffic direction within the proxy.',
    userSelectionTitle: 'User',
    userSelectionDescription: 'Select or specify an S3 user:',
    userSelectionPlaceholder: 'Select or enter an S3 user name',
    bucketSelectionTitle: 'Bucket Name',
    bucketSelectionDescription: 'Narrow down a policy to a specific bucket:',
    bucketSelectionAll: 'Apply to all buckets (including future ones)',
    bucketSelectionPlaceholder: 'Enter bucket name',
    selectFromAvailableBuckets: 'Or select from available cluster buckets:',
    loadingBuckets: 'Discovering buckets...',
    availableBucketsCount: '{count} buckets available',
    bucketSelectionInputLabel: 'Bucket Name',
    bucketNameRequired: 'Please enter a bucket name.',
    bucketNameInvalid:
      'Invalid bucket name. Use 3-63 lowercase letters, numbers, hyphens, or dots.',
    bucketErrLength: 'Bucket name must be between 3 and 63 characters long.',
    bucketErrChars:
      'Bucket name can only contain lowercase letters, numbers, dots, and hyphens.',
    bucketErrStartEnd:
      'Bucket name must begin and end with a letter or number.',
    bucketErrAdjacentPeriods:
      'Bucket name must not contain two adjacent periods.',
    bucketErrIpAddress: 'Bucket name must not be formatted as an IP address.',
    bucketErrPrefixSuffix:
      'Bucket name contains a restricted AWS prefix or suffix.',
    storageSelectionTitle: 'Storage',
    storageSelectionDescription: 'Select a destination storage:',
    storageSelectionRequired: 'Please select a destination storage.',
    storageSelectionBlockedOnlyHint:
      'No storage selected: a blocked-only policy without routing will be created.',
    unknownValidationError: 'Unknown validation error',
    statusSelectionTitle: 'Blocked Status',
    statusSelectionDescription:
      'Determine whether this routing policy should be actively allowed or explicitly blocked.',
    statusSelectionAllowed: 'Allowed',
    statusSelectionBlocked: 'Blocked',
    actionAddRoutingPolicy: 'Add Routing Policy',
    userSelectionRequired: 'Please select a user.',
    addRoutingPolicyConfirmTitle: 'Confirm Routing Policy Creation',
    confirmRoutingPolicyDescription:
      'You are about to create the following routing policy:',
    confirmRoutingPolicyPositive: 'Create Routing Policy',
    confirmRoutingPolicyNegative: 'Cancel',
    summaryColumnUser: 'User',
    summaryColumnStorage: 'Storage',
    summaryNoStoragePlaceholder: 'No storage selected',
    summaryColumnBucket: 'Bucket',
    summaryColumnStatus: 'Blocked Status',
    summaryStatusBlocked: 'Blocked',
    summaryStatusAllowed: 'Allowed',
    addRoutingPolicySuccessTitle: 'Routing Policy created!',
    addRoutingPolicySuccessContent:
      'Routing Policy has been created successfully.',
    addRoutingPolicyErrorTitle: 'Routing Policy not created!',
    addRoutingPolicyErrorContent:
      'An error occurred while creating the routing policy:',
    addErrorAction: 'Retry',
    addRoutingPolicyErrorUnknown:
      'An unknown error occurred while creating the routing policy.',
    addBlockErrorUnknown:
      'An unknown error occurred while creating a blocked policy',
    addBlockRollbackErrorUnknown:
      'Blocking the newly created routing policy failed. The routing policy creation has been rolled back. You can retry, or create the routing policy without blocking and block it separately afterwards.',
    addRoutingPolicyRollbackErrorUnknown:
      'Blocking the newly created routing policy failed, and rolling back the creation also failed. The routing policy was created but is not blocked. Please block it manually.',
  },
  [I18nLocale.DE]: {
    backToRoutingPolicies: 'Routing-Richtlinien',
    breadcrumbAddRoutingPolicy: 'Routing-Richtlinie hinzufügen',
    addRoutingPolicyTitle: 'Routing-Richtlinie hinzufügen',
    addRoutingPolicyHeader:
      'Neue Routing-Richtlinie erstellen, um den Datenverkehr im Proxy zu steuern.',
    userSelectionTitle: 'Benutzer',
    userSelectionDescription: 'Benutzer für die Routing-Richtlinie auswählen:',
    userSelectionPlaceholder: 'S3-Benutzernamen auswählen oder eingeben',
    bucketSelectionTitle: 'Bucket Name',
    bucketSelectionDescription:
      'Richtlinie auf einen speziellen Bucket anwenden:',
    bucketSelectionAll:
      'Auf alle Buckets anwenden (einschließlich zukünftiger)',
    bucketSelectionPlaceholder: 'Bucket Namen eingeben',
    selectFromAvailableBuckets:
      'Oder aus verfügbaren Cluster-Buckets auswählen:',
    loadingBuckets: 'Buckets werden ermittelt...',
    availableBucketsCount: '{count} Buckets verfügbar',
    bucketSelectionInputLabel: 'Bucket-Name',
    bucketNameRequired: 'Bitte geben Sie einen Bucket-Namen ein.',
    bucketNameInvalid:
      'Ungültiger Bucket-Name. Erlaubt sind 3-63 Kleinbuchstaben, Zahlen, Bindestriche oder Punkte.',
    bucketErrLength:
      'Der Bucket-Name muss zwischen 3 und 63 Zeichen lang sein.',
    bucketErrChars:
      'Der Bucket-Name darf nur Kleinbuchstaben, Zahlen, Punkte und Bindestriche enthalten.',
    bucketErrStartEnd:
      'Der Bucket-Name muss mit einem Buchstaben oder einer Zahl beginnen und enden.',
    bucketErrAdjacentPeriods:
      'Der Bucket-Name darf keine zwei aufeinanderfolgenden Punkte enthalten.',
    bucketErrIpAddress:
      'Der Bucket-Name darf nicht als IP-Adresse formatiert sein.',
    bucketErrPrefixSuffix:
      'Der Bucket-Name enthält ein unzulässiges AWS-Präfix oder -Suffix.',
    storageSelectionTitle: 'Speicher',
    storageSelectionDescription: 'Wählen Sie einen Zielspeicher:',
    storageSelectionRequired: 'Bitte wählen Sie einen Zielspeicher aus.',
    storageSelectionBlockedOnlyHint:
      'Kein Speicher ausgewählt: es wird nur eine blockierte Richtlinie erstellt.',
    unknownValidationError: 'Unbekannter Fehler beim Validieren der Eingabe',
    statusSelectionTitle: 'Blockiert-Status',
    statusSelectionDescription:
      'Legen Sie fest, ob diese Routing-Policy zugelassen oder explizit blockiert werden soll.',
    statusSelectionAllowed: 'Zugelassen',
    statusSelectionBlocked: 'Blockiert',
    actionAddRoutingPolicy: 'Routing-Richtlinie hinzufügen',
    userSelectionRequired: 'Bitte wählen Sie einen Benutzer aus.',
    addRoutingPolicyConfirmTitle: 'Anlegen der Routing-Richtlinie bestätigen',
    confirmRoutingPolicyDescription:
      'Sie erstellen folgende Routing-Richtlinie:',
    confirmRoutingPolicyPositive: 'Routing-Richtlinie erstellen',
    confirmRoutingPolicyNegative: 'Abbrechen',
    summaryColumnUser: 'Benutzer',
    summaryColumnStorage: 'Speicher',
    summaryNoStoragePlaceholder: 'Kein Speicher gewählt',
    summaryColumnBucket: 'Bucket',
    summaryColumnStatus: 'Blockiert-Status',
    summaryStatusBlocked: 'Blockiert',
    summaryStatusAllowed: 'Zugelassen',
    addRoutingPolicySuccessTitle: 'Routing-Richtlinie erstellt!',
    addRoutingPolicySuccessContent:
      'Routing-Richtlinie wurde erfolgreich erstellt.',
    addRoutingPolicyErrorTitle: 'Routing-Richtlinie nicht erstellt!',
    addRoutingPolicyErrorContent:
      'Beim Erstellen der Routing-Richtlinie ist ein Fehler aufgetreten:',
    addErrorAction: 'Erneut versuchen',
    addRoutingPolicyErrorUnknown:
      'Beim Erstellen der Routing-Richtlinie ist ein unbekannter Fehler aufgetreten.',
    addBlockErrorUnknown:
      'Beim Erstellen einer Blockierung ist ein unbekannter Fehler aufgetreten.',
    addBlockRollbackErrorUnknown:
      'Das Blockieren der neu erstellten Routing-Richtlinie ist fehlgeschlagen. Die Erstellung der Routing-Richtlinie wurde rückgängig gemacht. Sie können den Vorgang wiederholen oder die Routing-Richtlinie zunächst ohne Blockierung erstellen und sie anschließend separat blockieren.',
    addRoutingPolicyRollbackErrorUnknown:
      'Das Blockieren der neu erstellten Routing-Richtlinie ist fehlgeschlagen, und das Rückgängigmachen der Erstellung ist ebenfalls fehlgeschlagen. Die Routing-Richtlinie wurde erstellt, ist jedoch nicht blockiert. Bitte blockieren Sie sie manuell.',
  },
  vi: {
    backToRoutingPolicies: 'Routing policy',
    breadcrumbAddRoutingPolicy: 'Thêm routing policy',
    addRoutingPolicyTitle: 'Thêm routing policy',
    addRoutingPolicyHeader:
      'Tạo routing policy mới để điều khiển hướng lưu lượng trong proxy.',
    userSelectionTitle: 'User',
    userSelectionDescription: 'Chọn hoặc chỉ định một S3 user:',
    userSelectionPlaceholder: 'Chọn hoặc nhập tên S3 user',
    bucketSelectionTitle: 'Tên bucket',
    bucketSelectionDescription: 'Giới hạn policy cho một bucket cụ thể:',
    bucketSelectionAll: 'Áp dụng cho tất cả bucket (kể cả bucket tạo sau này)',
    bucketSelectionPlaceholder: 'Nhập tên bucket',
    selectFromAvailableBuckets: 'Hoặc chọn từ các bucket có sẵn trong cluster:',
    loadingBuckets: 'Đang tìm bucket...',
    availableBucketsCount: 'Có {count} bucket khả dụng',
    bucketSelectionInputLabel: 'Tên bucket',
    bucketNameRequired: 'Vui lòng nhập tên bucket.',
    bucketNameInvalid:
      'Tên bucket không hợp lệ. Dùng 3-63 ký tự gồm chữ thường, số, dấu gạch ngang hoặc dấu chấm.',
    bucketErrLength: 'Tên bucket phải dài từ 3 đến 63 ký tự.',
    bucketErrChars:
      'Tên bucket chỉ được chứa chữ thường, số, dấu chấm và dấu gạch ngang.',
    bucketErrStartEnd:
      'Tên bucket phải bắt đầu và kết thúc bằng chữ cái hoặc số.',
    bucketErrAdjacentPeriods:
      'Tên bucket không được chứa hai dấu chấm liền nhau.',
    bucketErrIpAddress: 'Tên bucket không được có định dạng địa chỉ IP.',
    bucketErrPrefixSuffix:
      'Tên bucket chứa tiền tố hoặc hậu tố bị AWS hạn chế.',
    storageSelectionTitle: 'Storage',
    storageSelectionDescription: 'Chọn storage đích:',
    storageSelectionRequired: 'Vui lòng chọn storage đích.',
    storageSelectionBlockedOnlyHint:
      'Chưa chọn storage: sẽ tạo policy chỉ chặn, không định tuyến.',
    unknownValidationError: 'Lỗi xác thực không xác định',
    statusSelectionTitle: 'Trạng thái chặn',
    statusSelectionDescription:
      'Xác định routing policy này được cho phép hoạt động hay bị chặn rõ ràng.',
    statusSelectionAllowed: 'Cho phép',
    statusSelectionBlocked: 'Bị chặn',
    actionAddRoutingPolicy: 'Thêm routing policy',
    userSelectionRequired: 'Vui lòng chọn user.',
    addRoutingPolicyConfirmTitle: 'Xác nhận tạo routing policy',
    confirmRoutingPolicyDescription: 'Bạn sắp tạo routing policy sau:',
    confirmRoutingPolicyPositive: 'Tạo routing policy',
    confirmRoutingPolicyNegative: 'Hủy',
    summaryColumnUser: 'User',
    summaryColumnStorage: 'Storage',
    summaryNoStoragePlaceholder: 'Chưa chọn storage',
    summaryColumnBucket: 'Bucket',
    summaryColumnStatus: 'Trạng thái chặn',
    summaryStatusBlocked: 'Bị chặn',
    summaryStatusAllowed: 'Cho phép',
    addRoutingPolicySuccessTitle: 'Đã tạo routing policy!',
    addRoutingPolicySuccessContent: 'Đã tạo routing policy thành công.',
    addRoutingPolicyErrorTitle: 'Chưa tạo được routing policy!',
    addRoutingPolicyErrorContent: 'Đã xảy ra lỗi khi tạo routing policy:',
    addErrorAction: 'Thử lại',
    addRoutingPolicyErrorUnknown:
      'Đã xảy ra lỗi không xác định khi tạo routing policy.',
    addBlockErrorUnknown: 'Đã xảy ra lỗi không xác định khi tạo policy chặn',
    addBlockRollbackErrorUnknown:
      'Chặn routing policy vừa tạo không thành công. Việc tạo routing policy đã được hoàn tác. Bạn có thể thử lại, hoặc tạo routing policy không chặn rồi chặn riêng sau đó.',
    addRoutingPolicyRollbackErrorUnknown:
      'Chặn routing policy vừa tạo không thành công, và việc hoàn tác cũng thất bại. Routing policy đã được tạo nhưng chưa bị chặn. Vui lòng chặn thủ công.',
  },
};
