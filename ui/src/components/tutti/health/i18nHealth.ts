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
// Plain locale codes (en, de, vi) keep this file free of the UI kit bundle.
export default <Record<string, Record<string, string>>>{
  en: {
    pageDescription:
      'Live availability of every storage as seen by each proxy, with the effective proxy settings.',
    title: 'Storage health',
    refresh: 'Refresh',
    proxy: 'Proxy',
    startedAt: 'Started',
    updatedAt: 'Last report',
    emptyTitle: 'No proxy data',
    empty:
      'No proxy has reported status in the last 30 s. Check that the proxies are running and can reach the Redis instance.',
    errorTitle: 'Failed to load storage health',
    errorText: 'The status of the proxies could not be loaded.',
    retry: 'Retry',
    columnStorage: 'Storage',
    columnStatus: 'Status',
    columnFailures: 'Consecutive failures',
    columnLastError: 'Last error',
    columnLastChange: 'Last change',
    available: 'Available',
    availableWarning: 'Available, with a warning',
    unavailable: 'Unavailable',
    settingsTitle: 'Effective settings',
    syncReplication: 'Synchronous replication',
    writeQuorum: 'Write quorum',
    syncTimeout: 'Sync timeout',
    onQuorumFailure: 'On quorum failure',
    failover: 'Automatic failover',
    failureThreshold: 'Health failure threshold',
    openDuration: 'Health open duration',
    requireSignedContentType: 'Require signed Content-Type',
    on: 'on',
    off: 'off',
    yes: 'yes',
    no: 'no',
  },
  de: {
    pageDescription:
      'Aktuelle Verfügbarkeit jedes Speichers aus Sicht der einzelnen Proxys, mit den wirksamen Proxy-Einstellungen.',
    title: 'Speicher-Zustand',
    refresh: 'Aktualisieren',
    proxy: 'Proxy',
    startedAt: 'Gestartet',
    updatedAt: 'Letzte Meldung',
    emptyTitle: 'Keine Proxy-Daten',
    empty:
      'Kein Proxy hat in den letzten 30 s einen Status gemeldet. Prüfen Sie, ob die Proxys laufen und die Redis-Instanz erreichen.',
    errorTitle: 'Speicher-Zustand konnte nicht geladen werden',
    errorText: 'Der Status der Proxys konnte nicht geladen werden.',
    retry: 'Erneut versuchen',
    columnStorage: 'Speicher',
    columnStatus: 'Status',
    columnFailures: 'Aufeinanderfolgende Fehler',
    columnLastError: 'Letzter Fehler',
    columnLastChange: 'Letzte Änderung',
    available: 'Verfügbar',
    availableWarning: 'Verfügbar, mit Warnung',
    unavailable: 'Nicht verfügbar',
    settingsTitle: 'Wirksame Einstellungen',
    syncReplication: 'Synchrone Replikation',
    writeQuorum: 'Schreib-Quorum',
    syncTimeout: 'Sync-Timeout',
    onQuorumFailure: 'Bei Quorum-Fehler',
    failover: 'Automatisches Failover',
    failureThreshold: 'Fehlerschwelle',
    openDuration: 'Sperrdauer',
    requireSignedContentType: 'Signierten Content-Type verlangen',
    on: 'an',
    off: 'aus',
    yes: 'ja',
    no: 'nein',
  },
  vi: {
    pageDescription:
      'Tình trạng từng storage theo góc nhìn của mỗi proxy, kèm cấu hình đang áp dụng.',
    title: 'Sức khỏe storage',
    refresh: 'Làm mới',
    proxy: 'Proxy',
    startedAt: 'Khởi động',
    updatedAt: 'Báo cáo gần nhất',
    emptyTitle: 'Chưa có dữ liệu proxy',
    empty:
      'Chưa có proxy nào báo trạng thái trong 30 giây qua. Kiểm tra proxy đang chạy và kết nối được tới Redis.',
    errorTitle: 'Không tải được sức khỏe storage',
    errorText: 'Không tải được trạng thái của các proxy.',
    retry: 'Thử lại',
    columnStorage: 'Storage',
    columnStatus: 'Trạng thái',
    columnFailures: 'Số lần lỗi liên tiếp',
    columnLastError: 'Lỗi gần nhất',
    columnLastChange: 'Thay đổi gần nhất',
    available: 'Sẵn sàng',
    availableWarning: 'Sẵn sàng, có cảnh báo',
    unavailable: 'Không khả dụng',
    settingsTitle: 'Cấu hình đang áp dụng',
    syncReplication: 'Sao chép đồng bộ',
    writeQuorum: 'Write quorum',
    syncTimeout: 'Timeout đồng bộ',
    onQuorumFailure: 'Khi quorum thất bại',
    failover: 'Failover tự động',
    failureThreshold: 'Ngưỡng lỗi sức khỏe',
    openDuration: 'Thời gian mở breaker',
    requireSignedContentType: 'Yêu cầu Content-Type đã ký',
    on: 'bật',
    off: 'tắt',
    yes: 'có',
    no: 'không',
  },
};
