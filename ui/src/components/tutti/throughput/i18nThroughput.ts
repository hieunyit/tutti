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
    title: 'Throughput',
    description:
      'Live copy rates of the replication workers. Updates every 5 seconds.',
    objects1m: 'Objects/s (1 min)',
    objects5m: 'Objects/s (5 min)',
    bytes1m: 'Throughput (1 min)',
    bytes5m: 'Throughput (5 min)',
    inFlight: 'In flight / concurrency',
    inFlightHint: 'Copies running now of all slots',
    buffer: 'Buffer used',
    bufferHint: '{used} of {total}',
    bufferNone: 'No buffer limit',
    waiters: 'Buffer waiters',
    waitersHint: 'Copies waiting for buffer space',
    failed: 'Failed copies (total)',
    workersTitle: 'Workers',
    colWorker: 'Worker',
    colInFlight: 'In flight',
    colObjects: 'Objects/s (1 min)',
    colBytes: 'Throughput (1 min)',
    colBuffer: 'Buffer',
    colFailed: 'Failed',
    diagnosticsTitle: 'What the numbers suggest',
    diagnosticsOk: 'No bottleneck detected right now.',
    hint_bufferBottleneck: 'Buffer budget is the bottleneck: raise it.',
    hint_workersBusy:
      'All workers are busy: raise concurrency if the CPU has headroom.',
    hint_workersIdle:
      'Workers are idle: check storage rate limits, S3 throttling (SlowDown) or the backlog.',
    hint_failuresRising: 'Failures are slowing copies: check the worker logs.',
    noWorkers: 'No worker has reported yet.',
    errorTitle: 'Throughput could not be loaded',
    errorText: 'The worker statistics request failed. Try again.',
    settingsLink: 'Tune in system settings',
  },
  [I18nLocale.DE]: {
    title: 'Durchsatz',
    description:
      'Live-Kopierraten der Replikations-Worker. Aktualisierung alle 5 Sekunden.',
    objects1m: 'Objekte/s (1 Min.)',
    objects5m: 'Objekte/s (5 Min.)',
    bytes1m: 'Durchsatz (1 Min.)',
    bytes5m: 'Durchsatz (5 Min.)',
    inFlight: 'In Arbeit / Parallelität',
    inFlightHint: 'Aktuell laufende Kopien aller Slots',
    buffer: 'Puffer belegt',
    bufferHint: '{used} von {total}',
    bufferNone: 'Kein Pufferlimit',
    waiters: 'Wartende auf Puffer',
    waitersHint: 'Kopien, die auf Pufferplatz warten',
    failed: 'Fehlgeschlagene Kopien (gesamt)',
    workersTitle: 'Worker',
    colWorker: 'Worker',
    colInFlight: 'In Arbeit',
    colObjects: 'Objekte/s (1 Min.)',
    colBytes: 'Durchsatz (1 Min.)',
    colBuffer: 'Puffer',
    colFailed: 'Fehlgeschlagen',
    diagnosticsTitle: 'Was die Zahlen nahelegen',
    diagnosticsOk: 'Aktuell kein Engpass erkennbar.',
    hint_bufferBottleneck: 'Das Pufferbudget ist der Engpass: Erhöhen Sie es.',
    hint_workersBusy:
      'Alle Worker sind ausgelastet: Erhöhen Sie die Parallelität, wenn die CPU Reserven hat.',
    hint_workersIdle:
      'Worker sind untätig: Prüfen Sie Speicher-Ratenlimits, S3-Drosselung (SlowDown) oder den Rückstau.',
    hint_failuresRising:
      'Fehler bremsen das Kopieren: Prüfen Sie die Worker-Logs.',
    noWorkers: 'Noch kein Worker hat sich gemeldet.',
    errorTitle: 'Durchsatz konnte nicht geladen werden',
    errorText:
      'Die Abfrage der Worker-Statistik ist fehlgeschlagen. Erneut versuchen.',
    settingsLink: 'In den Systemeinstellungen anpassen',
  },
  vi: {
    title: 'Thông lượng',
    description:
      'Tốc độ sao chép trực tiếp của các worker đồng bộ. Cập nhật mỗi 5 giây.',
    objects1m: 'Đối tượng/s (1 phút)',
    objects5m: 'Đối tượng/s (5 phút)',
    bytes1m: 'Thông lượng (1 phút)',
    bytes5m: 'Thông lượng (5 phút)',
    inFlight: 'Đang chạy / song song',
    inFlightHint: 'Số bản sao đang chạy trên tổng số slot',
    buffer: 'Bộ đệm đã dùng',
    bufferHint: '{used} / {total}',
    bufferNone: 'Không giới hạn bộ đệm',
    waiters: 'Đang chờ bộ đệm',
    waitersHint: 'Số bản sao chờ chỗ trong bộ đệm',
    failed: 'Bản sao thất bại (tổng)',
    workersTitle: 'Worker',
    colWorker: 'Worker',
    colInFlight: 'Đang chạy',
    colObjects: 'Đối tượng/s (1 phút)',
    colBytes: 'Thông lượng (1 phút)',
    colBuffer: 'Bộ đệm',
    colFailed: 'Thất bại',
    diagnosticsTitle: 'Các con số gợi ý',
    diagnosticsOk: 'Hiện chưa phát hiện nút thắt nào.',
    hint_bufferBottleneck: 'Ngân sách bộ đệm là nút thắt: hãy tăng lên.',
    hint_workersBusy:
      'Mọi worker đều bận: hãy tăng số tác vụ song song nếu CPU còn dư.',
    hint_workersIdle:
      'Worker đang rảnh: kiểm tra giới hạn tốc độ của storage, S3 throttling (SlowDown) hoặc hàng đợi tồn.',
    hint_failuresRising: 'Lỗi đang làm chậm việc sao chép: hãy kiểm tra log.',
    noWorkers: 'Chưa có worker nào báo cáo.',
    errorTitle: 'Không tải được thông lượng',
    errorText: 'Yêu cầu thống kê worker thất bại. Hãy thử lại.',
    settingsLink: 'Điều chỉnh trong cài đặt hệ thống',
  },
};
