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
    title: 'Add routing rule',
    loadError: 'Storages could not be loaded. Try again.',
    userLabel: 'User',
    userHint: 'Pick a user or type a name.',
    userRequired: 'Choose a user.',
    bucketsLegend: 'Buckets',
    bucketAll: 'All buckets, including future ones',
    bucketOne: 'One bucket',
    bucketLabel: 'Bucket name',
    bucketHint: 'Pick a bucket or type a name.',
    bucketsLoading: 'Looking for buckets…',
    bucketNameRequired: 'Enter a bucket name.',
    bucketErrLength: 'A bucket name has 3 to 63 characters.',
    bucketErrChars:
      'Use only lowercase letters, numbers, dots and hyphens in a bucket name.',
    bucketErrStartEnd: 'A bucket name starts and ends with a letter or number.',
    bucketErrAdjacentPeriods: 'A bucket name cannot contain two dots in a row.',
    bucketErrIpAddress: 'A bucket name cannot look like an IP address.',
    bucketErrPrefixSuffix: 'This bucket name uses a reserved prefix or suffix.',
    requestsLegend: 'Requests',
    requestsRoute: 'Send to a storage',
    requestsBlock: 'Block',
    blockHint: 'Requests are rejected until you allow the routing rule again.',
    storageLabel: 'Storage',
    storageRequired: 'Choose a storage.',
    create: 'Create routing rule',
    cancel: 'Cancel',
    created: 'Routing rule created',
    createFailed: 'The routing rule could not be created',
    createFailedHint: 'Your entries are kept. Check them and try again.',
    createErrorUnknown: 'The routing rule could not be created.',
    blockErrorUnknown: 'The routing rule could not be blocked.',
    rollbackErrorUnknown:
      'The routing rule was created but could not be blocked. Block it from the list.',
  },
  [I18nLocale.DE]: {
    title: 'Routing-Regel hinzufügen',
    loadError:
      'Die Speicher konnten nicht geladen werden. Versuchen Sie es erneut.',
    userLabel: 'Benutzer',
    userHint: 'Wählen Sie einen Benutzer oder geben Sie einen Namen ein.',
    userRequired: 'Wählen Sie einen Benutzer.',
    bucketsLegend: 'Buckets',
    bucketAll: 'Alle Buckets, auch künftige',
    bucketOne: 'Ein Bucket',
    bucketLabel: 'Bucket-Name',
    bucketHint: 'Wählen Sie einen Bucket oder geben Sie einen Namen ein.',
    bucketsLoading: 'Buckets werden gesucht …',
    bucketNameRequired: 'Geben Sie einen Bucket-Namen ein.',
    bucketErrLength: 'Ein Bucket-Name hat 3 bis 63 Zeichen.',
    bucketErrChars:
      'Ein Bucket-Name darf nur Kleinbuchstaben, Zahlen, Punkte und Bindestriche enthalten.',
    bucketErrStartEnd:
      'Ein Bucket-Name beginnt und endet mit einem Buchstaben oder einer Zahl.',
    bucketErrAdjacentPeriods:
      'Ein Bucket-Name darf nicht zwei Punkte hintereinander enthalten.',
    bucketErrIpAddress:
      'Ein Bucket-Name darf nicht wie eine IP-Adresse aussehen.',
    bucketErrPrefixSuffix:
      'Dieser Bucket-Name verwendet ein reserviertes Präfix oder Suffix.',
    requestsLegend: 'Anfragen',
    requestsRoute: 'An einen Speicher leiten',
    requestsBlock: 'Blockieren',
    blockHint:
      'Anfragen werden abgelehnt, bis Sie die Routing-Regel wieder zulassen.',
    storageLabel: 'Speicher',
    storageRequired: 'Wählen Sie einen Speicher.',
    create: 'Routing-Regel erstellen',
    cancel: 'Abbrechen',
    created: 'Routing-Regel erstellt',
    createFailed: 'Die Routing-Regel konnte nicht erstellt werden',
    createFailedHint:
      'Ihre Eingaben bleiben erhalten. Prüfen Sie sie und versuchen Sie es erneut.',
    createErrorUnknown: 'Die Routing-Regel konnte nicht erstellt werden.',
    blockErrorUnknown: 'Die Routing-Regel konnte nicht blockiert werden.',
    rollbackErrorUnknown:
      'Die Routing-Regel wurde erstellt, konnte aber nicht blockiert werden. Blockieren Sie sie in der Liste.',
  },
  vi: {
    title: 'Thêm routing rule',
    loadError: 'Không tải được danh sách storage. Hãy thử lại.',
    userLabel: 'User',
    userHint: 'Chọn một user hoặc nhập tên.',
    userRequired: 'Hãy chọn một user.',
    bucketsLegend: 'Bucket',
    bucketAll: 'Tất cả bucket, kể cả bucket tạo sau này',
    bucketOne: 'Một bucket',
    bucketLabel: 'Tên bucket',
    bucketHint: 'Chọn một bucket hoặc nhập tên.',
    bucketsLoading: 'Đang tìm bucket…',
    bucketNameRequired: 'Hãy nhập tên bucket.',
    bucketErrLength: 'Tên bucket dài từ 3 đến 63 ký tự.',
    bucketErrChars:
      'Tên bucket chỉ gồm chữ thường, số, dấu chấm và dấu gạch ngang.',
    bucketErrStartEnd: 'Tên bucket phải bắt đầu và kết thúc bằng chữ hoặc số.',
    bucketErrAdjacentPeriods: 'Tên bucket không có hai dấu chấm liền nhau.',
    bucketErrIpAddress: 'Tên bucket không được giống địa chỉ IP.',
    bucketErrPrefixSuffix:
      'Tên bucket này dùng tiền tố hoặc hậu tố dành riêng.',
    requestsLegend: 'Request',
    requestsRoute: 'Gửi tới một storage',
    requestsBlock: 'Chặn',
    blockHint: 'Request bị từ chối cho đến khi bạn cho phép lại routing rule.',
    storageLabel: 'Storage',
    storageRequired: 'Hãy chọn một storage.',
    create: 'Tạo routing rule',
    cancel: 'Hủy',
    created: 'Đã tạo routing rule',
    createFailed: 'Không thể tạo routing rule',
    createFailedHint: 'Các mục đã nhập được giữ lại. Hãy kiểm tra và thử lại.',
    createErrorUnknown: 'Không thể tạo routing rule.',
    blockErrorUnknown: 'Không thể chặn routing rule.',
    rollbackErrorUnknown:
      'Đã tạo routing rule nhưng không chặn được. Hãy chặn nó trong danh sách.',
  },
};
