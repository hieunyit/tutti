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
    statusInitial: 'Initial copy',
    statusLagging: 'Behind',
    pageDescription:
      'Continuously copy objects from a source storage to a destination, for a single bucket or for all buckets of a user.',
    replicationsTitle: 'Replication',
    noResultsTitle: 'No Replications yet',
    noResultsText:
      'There is no replications available at the moment.\nTo create a new replication please click the button below.',
    filterNoResultsTitle: 'No Replications found',
    filterNoResultsText:
      'There is no replication matching your filter criteria.',
    filterNoResultsAction: 'Clear Filters',
    errorTitle: 'Error',
    errorText:
      'An error occurred while getting the replication list.\nPlease try one more time.',
    errorAction: 'Retry',
    cancel: 'Cancel',
    columnUser: 'User',
    columnStatus: 'Status',
    columnActions: 'Actions',
    statusPaused: 'Paused',
    statusActive: 'Active',
    statusDone: 'Done',
    statusUptodate: 'Up to date',
    pauseSuccessTitle: 'Paused!',
    pauseErrorTitle: 'Pause failed!',
    resumeSuccessTitle: 'Resumed!',
    resumeErrorTitle: 'Resume failed!',
    deleteSuccessTitle: 'Deleted!',
    deleteErrorTitle: 'Deletion failed!',
    resumeSelectedSuccessContent:
      'The following {total} replications were resumed:',
    resumeSelectedErrorContent:
      'And error occurred while resuming the following {total} replications:',
    pauseSelectedSuccessContent:
      'The following {total} replications were paused:',
    pauseSelectedErrorContent:
      'And error occurred while pausing the following {total} replications:',
    deleteSelectedSuccessContent:
      'The following {total} replications were deleted:',
    deleteSelectedErrorContent:
      'And error occurred while deleting the following {total} replications:',
    filterStatusPaused: 'Paused',
    filterStatusActive: 'Active',
    filterStatusInitialDone: 'Initial copy done',
    filterStatusInitialInProgress: 'Initial copy running',
    filterStatusLiveUpToDate: 'Up to date',
    filterStatusLiveBehind: 'Behind',
    filterTypeBucket: 'Bucket replication',
    filterTypeUser: 'All buckets of a user',
    eventsBehind: '{n} events behind',
    filterLabelType: 'Type',
    filterLabelUser: 'User',
    filterLabelBucket: 'Bucket',
    filterLabelDirection: 'Storage',
    filterLabelStatus: 'Status',
    filterAny: 'Any',
    moreActions: 'More actions',
    initialCopy: 'Initial copy',
    backlog: 'Live changes',
    timeLeft: 'About {time} left',
    createdAt: 'Created',
  },
  [I18nLocale.DE]: {
    statusInitial: 'Erstkopie',
    statusLagging: 'Im Rückstand',
    pageDescription:
      'Kopiert Objekte fortlaufend von einem Quellspeicher in ein Ziel, für einen einzelnen Bucket oder alle Buckets eines Benutzers.',
    replicationsTitle: 'Replikation',
    noResultsTitle: 'Noch keine Replikationen',
    noResultsText:
      'Derzeit sind keine Replikationen verfügbar.\nUm eine neue Replikation zu erstellen, klicken Sie bitte auf die Schaltfläche unten.',
    filterNoResultsTitle: 'Keine Replikationen gefunden',
    filterNoResultsText:
      'Es gibt keine Replikationen, die Ihren Filterkriterien entsprechen.',
    filterNoResultsAction: 'Filter zurücksetzen',
    errorTitle: 'Fehler',
    errorText:
      'Beim Abrufen der Replikationsliste ist ein Fehler aufgetreten.\nBitte versuchen Sie es noch einmal.',
    errorAction: 'Erneut versuchen',
    cancel: 'Abbrechen',
    columnUser: 'Benutzer',
    columnStatus: 'Status',
    columnActions: 'Aktionen',
    statusPaused: 'Pausiert',
    statusActive: 'Aktiv',
    statusDone: 'Abgeschlossen',
    statusUptodate: 'Aktuell',
    pauseSuccessTitle: 'Pausiert!',
    pauseErrorTitle: 'Pausieren fehlgeschlagen!',
    resumeSuccessTitle: 'Fortgesetzt!',
    resumeErrorTitle: 'Fortsetzen fehlgeschlagen!',
    deleteSuccessTitle: 'Gelöscht!',
    deleteErrorTitle: 'Löschen fehlgeschlagen!',
    resumeSelectedSuccessContent:
      'Die folgenden {total} Replikationen wurden fortgesetzt:',
    resumeSelectedErrorContent:
      'Es ist ein Fehler beim Fortsetzen der folgenden {total} Replikationen aufgetreten:',
    pauseSelectedSuccessContent:
      'Die folgenden {total} Replikationen wurden pausiert:',
    pauseSelectedErrorContent:
      'Es ist ein Fehler beim Pausieren der folgenden {total} Replikationen aufgetreten:',
    deleteSelectedSuccessContent:
      'Die folgenden {total} Replikationen wurden gelöscht:',
    deleteSelectedErrorContent:
      'Es ist ein Fehler beim Löschen der folgenden {total} Replikationen aufgetreten:',
    filterStatusPaused: 'Pausiert',
    filterStatusActive: 'Aktiv',
    filterStatusInitialDone: 'Erstkopie abgeschlossen',
    filterStatusInitialInProgress: 'Erstkopie läuft',
    filterStatusLiveUpToDate: 'Aktuell',
    filterStatusLiveBehind: 'Im Rückstand',
    filterTypeBucket: 'Bucket-Replikation',
    filterTypeUser: 'Alle Buckets eines Benutzers',
    eventsBehind: '{n} Ereignisse im Rückstand',
    filterLabelType: 'Typ',
    filterLabelUser: 'Benutzer',
    filterLabelBucket: 'Bucket',
    filterLabelDirection: 'Speicher',
    filterLabelStatus: 'Status',
    filterAny: 'Alle',
    moreActions: 'Weitere Aktionen',
    initialCopy: 'Erstkopie',
    backlog: 'Laufende Änderungen',
    timeLeft: 'Noch etwa {time}',
    createdAt: 'Erstellt',
  },
  vi: {
    statusInitial: 'Sao chép ban đầu',
    statusLagging: 'Đang chậm',
    pageDescription:
      'Liên tục sao chép object từ storage nguồn sang đích, cho một bucket hoặc cho tất cả bucket của một user.',
    replicationsTitle: 'Replication',
    noResultsTitle: 'Chưa có replication',
    noResultsText:
      'Hiện chưa có replication nào.\nĐể tạo replication mới, hãy nhấp nút bên dưới.',
    filterNoResultsTitle: 'Không tìm thấy replication',
    filterNoResultsText: 'Không có replication nào khớp với tiêu chí lọc.',
    filterNoResultsAction: 'Xóa bộ lọc',
    errorTitle: 'Lỗi',
    errorText:
      'Đã xảy ra lỗi khi lấy danh sách replication.\nVui lòng thử lại một lần nữa.',
    errorAction: 'Thử lại',
    cancel: 'Hủy',
    columnUser: 'User',
    columnStatus: 'Trạng thái',
    columnActions: 'Thao tác',
    statusPaused: 'Tạm dừng',
    statusActive: 'Hoạt động',
    statusDone: 'Hoàn tất',
    statusUptodate: 'Đã cập nhật mới nhất',
    pauseSuccessTitle: 'Đã tạm dừng!',
    pauseErrorTitle: 'Tạm dừng thất bại!',
    resumeSuccessTitle: 'Đã tiếp tục!',
    resumeErrorTitle: 'Tiếp tục thất bại!',
    deleteSuccessTitle: 'Đã xóa!',
    deleteErrorTitle: 'Xóa thất bại!',
    resumeSelectedSuccessContent: 'Đã tiếp tục {total} replication sau:',
    resumeSelectedErrorContent:
      'Đã xảy ra lỗi khi tiếp tục {total} replication sau:',
    pauseSelectedSuccessContent: 'Đã tạm dừng {total} replication sau:',
    pauseSelectedErrorContent:
      'Đã xảy ra lỗi khi tạm dừng {total} replication sau:',
    deleteSelectedSuccessContent: 'Đã xóa {total} replication sau:',
    deleteSelectedErrorContent:
      'Đã xảy ra lỗi khi xóa {total} replication sau:',
    filterStatusPaused: 'Tạm dừng',
    filterStatusActive: 'Hoạt động',
    filterStatusInitialDone: 'Sao chép ban đầu xong',
    filterStatusInitialInProgress: 'Đang sao chép ban đầu',
    filterStatusLiveUpToDate: 'Cập nhật',
    filterStatusLiveBehind: 'Đang chậm',
    filterTypeBucket: 'Bucket replication',
    filterTypeUser: 'Tất cả bucket của một user',
    eventsBehind: '{n} sự kiện chưa xử lý',
    filterLabelType: 'Loại',
    filterLabelUser: 'User',
    filterLabelBucket: 'Bucket',
    filterLabelDirection: 'Storage',
    filterLabelStatus: 'Trạng thái',
    filterAny: 'Tất cả',
    moreActions: 'Thao tác khác',
    initialCopy: 'Sao chép ban đầu',
    backlog: 'Thay đổi trực tiếp',
    timeLeft: 'Còn khoảng {time}',
    createdAt: 'Đã tạo',
  },
};
