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
    replicationsTitle: 'Replication',
    noResultsTitle: 'No Replications yet',
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
      'The following replication was resumed: | The following {total} replications were resumed:',
    resumeSelectedErrorContent:
      'An error occurred while resuming the following replication: | An error occurred while resuming the following {total} replications:',
    allBuckets: 'all buckets',
    pauseSelectedSuccessContent:
      'The following replication was paused: | The following {total} replications were paused:',
    pauseSelectedErrorContent:
      'An error occurred while pausing the following replication: | An error occurred while pausing the following {total} replications:',
    deleteSelectedSuccessContent:
      'The following replication was deleted: | The following {total} replications were deleted:',
    deleteSelectedErrorContent:
      'An error occurred while deleting the following replication: | An error occurred while deleting the following {total} replications:',
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
    replicationsTitle: 'Replikation',
    noResultsTitle: 'Noch keine Replikationen',
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
      'Die folgende Replikation wurde fortgesetzt: | Die folgenden {total} Replikationen wurden fortgesetzt:',
    resumeSelectedErrorContent:
      'Es ist ein Fehler beim Fortsetzen der folgenden Replikation aufgetreten: | Es ist ein Fehler beim Fortsetzen der folgenden {total} Replikationen aufgetreten:',
    allBuckets: 'alle Buckets',
    pauseSelectedSuccessContent:
      'Die folgende Replikation wurde pausiert: | Die folgenden {total} Replikationen wurden pausiert:',
    pauseSelectedErrorContent:
      'Es ist ein Fehler beim Pausieren der folgenden Replikation aufgetreten: | Es ist ein Fehler beim Pausieren der folgenden {total} Replikationen aufgetreten:',
    deleteSelectedSuccessContent:
      'Die folgende Replikation wurde gelöscht: | Die folgenden {total} Replikationen wurden gelöscht:',
    deleteSelectedErrorContent:
      'Es ist ein Fehler beim Löschen der folgenden Replikation aufgetreten: | Es ist ein Fehler beim Löschen der folgenden {total} Replikationen aufgetreten:',
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
    replicationsTitle: 'Replication',
    noResultsTitle: 'Chưa có replication',
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
    allBuckets: 'tất cả bucket',
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
