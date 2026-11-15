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
    title: 'Create diff report',
    loadError: 'Storages could not be loaded. Try again.',
    notEnoughStorages:
      'A diff report needs two storages. Add another storage, then try again.',
    stepLocations: 'Source and destination',
    stepChecks: 'Checks',
    sourceStorage: 'Source storage',
    sourceBucket: 'Source bucket',
    destinationStorage: 'Destination storage',
    destinationBucket: 'Destination bucket',
    bucketPlaceholder: 'my-bucket',
    storageRequired: 'Choose a storage.',
    bucketRequired: 'Enter a bucket name.',
    bucketErrLength: 'A bucket name has 3 to 63 characters.',
    bucketErrChars:
      'Use only lowercase letters, numbers, dots and hyphens in a bucket name.',
    bucketErrStartEnd: 'A bucket name starts and ends with a letter or number.',
    bucketErrAdjacentPeriods: 'A bucket name cannot contain two dots in a row.',
    bucketErrIpAddress: 'A bucket name cannot look like an IP address.',
    bucketErrPrefixSuffix: 'This bucket name uses a reserved prefix or suffix.',
    userLabel: 'User',
    userHint: 'Reads both buckets. Only users on both storages are listed.',
    userRequired: 'Choose a user.',
    noUsers:
      'No user exists on both storages. Add the user to the other storage, then try again.',
    checksLegend: 'Checks',
    lastVersions: 'Compare only the latest version of each object',
    lastVersionsHint: 'Turn off to compare older versions too.',
    ignoreEtags: 'Ignore ETags',
    ignoreEtagsHint:
      'Skips the checksum comparison. Useful when the storages calculate hashes differently.',
    ignoreSizes: 'Ignore sizes',
    ignoreSizesHint:
      'Skips the size comparison. Useful when objects are transformed or encrypted on the way.',
    summarySource: 'Source',
    summaryDestination: 'Destination',
    create: 'Create diff report',
    created: 'Diff report created',
    createFailed: 'The diff report could not be created',
    createFailedHint: 'Your entries are kept. Check them and try again.',
    unknownError: 'The server gave no reason.',
  },
  [I18nLocale.DE]: {
    title: 'Diff-Report erstellen',
    loadError:
      'Die Speicher konnten nicht geladen werden. Versuchen Sie es erneut.',
    notEnoughStorages:
      'Ein Diff-Report braucht zwei Speicher. Fügen Sie einen weiteren Speicher hinzu und versuchen Sie es erneut.',
    stepLocations: 'Quelle und Ziel',
    stepChecks: 'Prüfungen',
    sourceStorage: 'Quellspeicher',
    sourceBucket: 'Quell-Bucket',
    destinationStorage: 'Zielspeicher',
    destinationBucket: 'Ziel-Bucket',
    bucketPlaceholder: 'mein-bucket',
    storageRequired: 'Wählen Sie einen Speicher.',
    bucketRequired: 'Geben Sie einen Bucket-Namen ein.',
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
    userLabel: 'Benutzer',
    userHint:
      'Liest beide Buckets. Es werden nur Benutzer auf beiden Speichern angezeigt.',
    userRequired: 'Wählen Sie einen Benutzer.',
    noUsers:
      'Kein Benutzer existiert auf beiden Speichern. Fügen Sie den Benutzer zum anderen Speicher hinzu und versuchen Sie es erneut.',
    checksLegend: 'Prüfungen',
    lastVersions: 'Nur die neueste Version jedes Objekts vergleichen',
    lastVersionsHint: 'Ausschalten, um auch ältere Versionen zu vergleichen.',
    ignoreEtags: 'ETags ignorieren',
    ignoreEtagsHint:
      'Überspringt den Prüfsummenvergleich. Nützlich, wenn die Speicher Hashes unterschiedlich berechnen.',
    ignoreSizes: 'Größen ignorieren',
    ignoreSizesHint:
      'Überspringt den Größenvergleich. Nützlich, wenn Objekte unterwegs umgewandelt oder verschlüsselt werden.',
    summarySource: 'Quelle',
    summaryDestination: 'Ziel',
    create: 'Diff-Report erstellen',
    created: 'Diff-Report erstellt',
    createFailed: 'Der Diff-Report konnte nicht erstellt werden',
    createFailedHint:
      'Ihre Eingaben bleiben erhalten. Prüfen Sie sie und versuchen Sie es erneut.',
    unknownError: 'Der Server hat keinen Grund genannt.',
  },
  vi: {
    title: 'Tạo báo cáo diff',
    loadError: 'Không tải được danh sách storage. Hãy thử lại.',
    notEnoughStorages:
      'Báo cáo diff cần hai storage. Hãy thêm một storage nữa rồi thử lại.',
    stepLocations: 'Nguồn và đích',
    stepChecks: 'Kiểm tra',
    sourceStorage: 'Storage nguồn',
    sourceBucket: 'Bucket nguồn',
    destinationStorage: 'Storage đích',
    destinationBucket: 'Bucket đích',
    bucketPlaceholder: 'my-bucket',
    storageRequired: 'Hãy chọn một storage.',
    bucketRequired: 'Hãy nhập tên bucket.',
    bucketErrLength: 'Tên bucket dài từ 3 đến 63 ký tự.',
    bucketErrChars:
      'Tên bucket chỉ gồm chữ thường, số, dấu chấm và dấu gạch ngang.',
    bucketErrStartEnd: 'Tên bucket phải bắt đầu và kết thúc bằng chữ hoặc số.',
    bucketErrAdjacentPeriods: 'Tên bucket không có hai dấu chấm liền nhau.',
    bucketErrIpAddress: 'Tên bucket không được giống địa chỉ IP.',
    bucketErrPrefixSuffix:
      'Tên bucket này dùng tiền tố hoặc hậu tố dành riêng.',
    userLabel: 'User',
    userHint: 'Đọc cả hai bucket. Chỉ hiện user có trên cả hai storage.',
    userRequired: 'Hãy chọn một user.',
    noUsers:
      'Không có user nào tồn tại trên cả hai storage. Hãy thêm user vào storage còn lại rồi thử lại.',
    checksLegend: 'Kiểm tra',
    lastVersions: 'Chỉ so sánh phiên bản mới nhất của mỗi object',
    lastVersionsHint: 'Tắt để so sánh cả các phiên bản cũ.',
    ignoreEtags: 'Bỏ qua ETag',
    ignoreEtagsHint:
      'Bỏ qua so sánh checksum. Hữu ích khi các storage tính hash khác nhau.',
    ignoreSizes: 'Bỏ qua kích thước',
    ignoreSizesHint:
      'Bỏ qua so sánh kích thước. Hữu ích khi object bị biến đổi hoặc mã hóa trên đường truyền.',
    summarySource: 'Nguồn',
    summaryDestination: 'Đích',
    create: 'Tạo báo cáo diff',
    created: 'Đã tạo báo cáo diff',
    createFailed: 'Không thể tạo báo cáo diff',
    createFailedHint: 'Các mục đã nhập được giữ lại. Hãy kiểm tra và thử lại.',
    unknownError: 'Server không nêu lý do.',
  },
};
