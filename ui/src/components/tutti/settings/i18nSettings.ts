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

import type { I18nMessages } from '@clyso/clyso-ui-kit';

export default <I18nMessages>{
  en: {
    retry: 'Retry',
    diffTitle: 'Review changes before saving',
    diffText: 'These settings will be saved as version {version}.',
    diffNoChanges: 'No changes.',
    confirmSave: 'Save settings',
    cancel: 'Cancel',
    conflictTitle: 'Settings were changed by someone else',
    conflictText:
      'Another operator saved a newer version. Reload to see their values (your draft is discarded), or overwrite to save your draft on top of the newer version.',
    reload: 'Reload settings',
    overwrite: 'Overwrite with my draft',
    unsavedBar: 'You have unsaved changes',
    changesCount: '{count} change(s)',
    bannerFailed: 'Could not save settings',
    configuration: 'Configuration',
    settingsSections: 'System settings sections',
    tabFeatures: 'Features',
    tabBulk: 'Bulk migration',
    tabWorker: 'Worker',
    tabLocking: 'Locking',
    tabWebhook: 'Webhook',
    restartRequired: 'Requires worker restart',
    saved: 'Saved',
    validationSummary: '{count} field(s) need attention',
    reviewFields: 'Review fields',
    loading: 'Loading configuration…',
    savedVersion: 'Saved version',
    effectiveNotice:
      'Saved values may take a moment to reach workers. Worker concurrency and intervals require a restart.',
    bulkTitle: 'Bulk migration limits',
    bulkDesc:
      'Control copy throughput, multipart buffer usage and queue backpressure per worker and copy queue.',
    requestFailed: 'Configuration request failed',
    loadFailed:
      'The configuration could not be loaded. Retry before editing; no defaults have been applied.',
    draftRetained:
      'Your draft is retained. Refresh to load the current version before saving again.',
    discardTitle: 'Discard unsaved changes?',
    discardText:
      'Your draft will be replaced or left behind. Continue only if you no longer need these edits.',
    discardConfirm: 'Discard changes',
    keepEditing: 'Keep editing',
    durationHint: 'Duration format: 500ms, 30s or 1m30s.',
    invalidDuration: 'Enter a positive duration, e.g. 500ms, 30s or 1m30s.',
    invalidConcurrency: 'Enter a whole number between 1 and 1,000.',
    invalidWatermark: 'Enter a whole number between 1 and 2,147,483,647.',
    invalidWatermarkOrder:
      'The low watermark must be smaller than the high watermark.',
    invalidBytes:
      'Enter a non-negative whole number of bytes (maximum 9,223,372,036,854,775,807).',
    invalidUrl:
      'Enter an absolute HTTP(S) URL without embedded credentials, or leave it empty.',
    title: 'System settings',
    subtitle: 'Manage replication features and worker configuration.',
    lastUpdated: 'Last updated',
    version: 'Version',
    save: 'Save settings',
    saving: 'Saving...',
    reset: 'Discard changes',
    refresh: 'Refresh',
    featuresTitle: 'Replication features',
    featuresDesc:
      'Choose which object and bucket metadata to replicate between storages.',
    tagging: 'Object & bucket tagging',
    taggingDesc:
      'Synchronize user tags assigned to buckets and objects across storages',
    acl: 'Access control lists (ACL)',
    aclDesc:
      'Synchronize object and bucket permissions and access grants across storages',
    lifecycle: 'Lifecycle configuration',
    lifecycleDesc:
      'Synchronize bucket lifecycle expiration and transition rules',
    policy: 'Bucket policies',
    policyDesc:
      'Synchronize JSON bucket access policies across member storages',
    preserveAclGrants: 'Preserve source ACL grants',
    preserveAclGrantsDesc:
      'Maintain original grant permissions rather than mapping to target owner',
    versioning: 'Object versioning',
    versioningDesc:
      'Synchronize versioned objects and delete markers across storages',
    workerTitle: 'Worker configuration',
    workerDesc:
      'Set task concurrency and processing intervals. Changes in this section require a worker restart.',
    concurrency: 'Task concurrency',
    concurrencyHint: 'Maximum parallel replication tasks per worker process.',
    shutdownTimeout: 'Shutdown timeout',
    queueUpdateInterval: 'Queue update interval',
    taskCheckInterval: 'Task check interval',
    delayedTaskCheckInterval: 'Delayed task check interval',
    swiftRetryInterval: 'Swift retry interval',
    pauseRetryInterval: 'Pause retry interval',
    switchRetryInterval: 'Switch retry interval',
    backlogHighWatermark: 'High watermark (tasks)',
    backlogHighWatermarkHint:
      'Pause listing when the copy backlog reaches this limit.',
    backlogLowWatermark: 'Low watermark (tasks)',
    backlogLowWatermarkHint:
      'Resume listing below this limit. Must be lower than the high watermark.',
    bandwidthLimitBytes: 'Copy bandwidth (bytes/s)',
    bandwidthLimitHint: 'Per worker. 0 = unlimited; 2097152 = 2 MiB/s.',
    bufferBudgetBytes: 'Multipart buffer budget (bytes)',
    bufferBudgetHint: 'Per worker. 0 = unlimited; 268435456 = 256 MiB.',
    tuneTitle: 'Replication throughput',
    tuneDesc:
      'Tune how many copies run in parallel and how multipart uploads are split. Leave a field blank (or 0) to use the default shown in it.',
    needsRestart: 'Needs worker restart',
    appliesLive: 'Applies immediately',
    bulkConcurrencyPercent: 'Bulk concurrency share (%)',
    bulkConcurrencyPercentHint:
      'Share of task concurrency reserved for bulk migration. Default 20; allowed 5 to 95.',
    multipartThreads: 'Multipart threads',
    multipartThreadsHint:
      'Parallel part uploads per object. Default 1; allowed 1 to 16.',
    multipartPartSizeMiB: 'Multipart part size (MiB)',
    multipartPartSizeMiBHint:
      'Size of each uploaded part. Default 16; allowed 5 to 512.',
    invalidBulkPercent: 'Enter 0 (default) or a whole number from 5 to 95.',
    invalidMultipartThreads:
      'Enter 0 (default) or a whole number from 1 to 16.',
    invalidPartSize: 'Enter 0 (default) or a whole number from 5 to 512.',
    recTitle: 'Recommended values',
    recIntro:
      'Starting points for your hardware. Nothing is applied until you press Fill in and save.',
    recCores: 'CPU cores per worker',
    recRam: 'RAM per worker (GB)',
    recFill: 'Fill in',
    recConcurrency: 'Task concurrency: about 16 per core, at most 256.',
    recBuffer: 'Buffer budget: 25% of RAM, at most 8 GiB.',
    recThreads: 'Multipart threads: 4 for object storage over a WAN.',
    recBulk: 'Bulk share: 80% when live proxy traffic is light.',
    throughputLink: 'Open throughput panel',
    lockTitle: 'Distributed locking',
    lockDesc: 'Configure the overlap margin for distributed worker locks.',
    lockOverlap: 'Lock overlap',
    lockOverlapHint: 'Duration margin for distributed locks, e.g. 2s.',
    webhookTitle: 'Webhook configuration',
    webhookDesc: 'Receive object notifications from S3 and Swift storages.',
    webhookEnabled: 'Enable webhook endpoint',
    webhookBaseUrl: 'Webhook base URL',
    webhookBaseUrlHint:
      'Public HTTP(S) address for incoming storage notifications.',
    saveSuccess:
      'Configuration saved. Live settings propagate asynchronously; concurrency and interval changes require a worker restart. Runtime application has not been confirmed by this page.',
    conflictError:
      'The configuration was modified by another operator. Reload and review your changes.',
    unsavedChanges: 'Unsaved changes',
    never: 'Never',
  },
  de: {
    retry: 'Erneut versuchen',
    diffTitle: 'Änderungen vor dem Speichern prüfen',
    diffText: 'Diese Einstellungen werden als Version {version} gespeichert.',
    diffNoChanges: 'Keine Änderungen.',
    confirmSave: 'Einstellungen speichern',
    cancel: 'Abbrechen',
    conflictTitle: 'Einstellungen wurden von jemand anderem geändert',
    conflictText:
      'Ein anderer Operator hat eine neuere Version gespeichert. Neu laden, um dessen Werte zu sehen (Ihr Entwurf wird verworfen), oder überschreiben, um Ihren Entwurf auf der neueren Version zu speichern.',
    reload: 'Einstellungen neu laden',
    overwrite: 'Mit meinem Entwurf überschreiben',
    unsavedBar: 'Es gibt ungespeicherte Änderungen',
    changesCount: '{count} Änderung(en)',
    bannerFailed: 'Einstellungen konnten nicht gespeichert werden',
    configuration: 'Konfiguration',
    settingsSections: 'Bereiche der Systemeinstellungen',
    tabFeatures: 'Funktionen',
    tabBulk: 'Massenmigration',
    tabWorker: 'Worker',
    tabLocking: 'Sperren',
    tabWebhook: 'Webhook',
    restartRequired: 'Worker-Neustart erforderlich',
    saved: 'Gespeichert',
    validationSummary: '{count} Feld(er) prüfen',
    reviewFields: 'Felder prüfen',
    loading: 'Konfiguration wird geladen…',
    savedVersion: 'Gespeicherte Version',
    effectiveNotice:
      'Gespeicherte Werte erreichen die Worker zeitversetzt. Parallelität und Intervalle erfordern einen Neustart.',
    bulkTitle: 'Limits für Massenmigration',
    bulkDesc:
      'Live-Limits je Worker-Prozess und Kopierwarteschlange, keine clusterweiten Ressourcenlimits.',
    requestFailed: 'Konfigurationsanfrage fehlgeschlagen',
    loadFailed:
      'Konfiguration konnte nicht geladen werden. Vor dem Bearbeiten erneut versuchen; es wurden keine Standardwerte angewendet.',
    draftRetained:
      'Ihr Entwurf bleibt erhalten. Vor dem erneuten Speichern die aktuelle Version laden.',
    discardTitle: 'Ungespeicherte Änderungen verwerfen?',
    discardText:
      'Der Entwurf wird ersetzt oder verlassen. Nur fortfahren, wenn diese Änderungen nicht mehr benötigt werden.',
    discardConfirm: 'Änderungen verwerfen',
    keepEditing: 'Weiter bearbeiten',
    durationHint: 'Dauerformat: 500ms, 30s oder 1m30s.',
    invalidDuration: 'Positive Dauer eingeben, z. B. 500ms, 30s oder 1m30s.',
    invalidConcurrency: 'Ganze Zahl zwischen 1 und 1.000 eingeben.',
    invalidWatermark: 'Ganze Zahl zwischen 1 und 2.147.483.647 eingeben.',
    invalidWatermarkOrder: 'Die untere Grenze muss kleiner als die obere sein.',
    invalidBytes:
      'Nichtnegative ganze Bytezahl eingeben (maximal 9.223.372.036.854.775.807).',
    invalidUrl:
      'Absolute HTTP(S)-URL ohne Zugangsdaten eingeben oder leer lassen.',
    title: 'Systemeinstellungen',
    subtitle:
      'Cluster-Funktionen, Worker-Leistungsoptimierung und Betriebsparameter',
    lastUpdated: 'Zuletzt aktualisiert',
    version: 'Version',
    save: 'Einstellungen speichern',
    saving: 'Wird gespeichert...',
    reset: 'Änderungen verwerfen',
    refresh: 'Aktualisieren',
    featuresTitle: 'Replikationsfunktionen',
    featuresDesc:
      'Optionale Replikationsfunktionen clusterweit ohne Dienstneustart aktivieren oder deaktivieren',
    tagging: 'Objekt- & Bucket-Tagging',
    taggingDesc:
      'Benutzer-Tags für Buckets und Objekte über Speicher hinweg synchronisieren',
    acl: 'Zugriffskontrolllisten (ACL)',
    aclDesc:
      'Objekt- und Bucket-Berechtigungen über Speicher hinweg synchronisieren',
    lifecycle: 'Lebenszyklus-Konfiguration',
    lifecycleDesc: 'Bucket-Ablauf- und Übergangsregeln synchronisieren',
    policy: 'Bucket-Richtlinien',
    policyDesc:
      'JSON-Bucket-Zugriffsrichtlinien zwischen Speichern synchronisieren',
    preserveAclGrants: 'Quell-ACL-Berechtigungen beibehalten',
    preserveAclGrantsDesc:
      'Ursprüngliche Berechtigungen beibehalten statt auf Zielbesitzer abzubilden',
    versioning: 'Objekt-Versionierung',
    versioningDesc:
      'Versionierte Objekte und Löschmarkierungen synchronisieren',
    workerTitle: 'Worker-Konfiguration',
    workerDesc:
      'Parallelität und Verarbeitungsintervalle festlegen. Änderungen erfordern einen Worker-Neustart.',
    concurrency: 'Task-Parallelität',
    concurrencyHint:
      'Maximale Anzahl paralleler Replikationsaufgaben je Worker-Prozess.',
    shutdownTimeout: 'Timeout beim Herunterfahren',
    queueUpdateInterval: 'Warteschlangen-Aktualisierungsintervall',
    taskCheckInterval: 'Task-Prüfintervall',
    delayedTaskCheckInterval: 'Intervall verzögerter Tasks',
    swiftRetryInterval: 'Swift-Wiederholungsintervall',
    pauseRetryInterval: 'Pausen-Wiederholungsintervall',
    switchRetryInterval: 'Umschalt-Wiederholungsintervall',
    backlogHighWatermark: 'Backlog-Höchstgrenze (Tasks)',
    backlogHighWatermarkHint:
      'Wenn anstehende Kopieraufgaben diese Grenze überschreiten, pausiert die Auflistung (live angewendet)',
    backlogLowWatermark: 'Backlog-Niedriggrenze (Tasks)',
    backlogLowWatermarkHint:
      'Setzt die normale Auflistung fort, sobald die Warteschlange unter diese Grenze fällt (live angewendet)',
    bandwidthLimitBytes: 'Massenkopie-Bandbreite (Bytes/s)',
    bandwidthLimitHint:
      'Bandbreite drosseln (z.B. 2097152 für 2 MiB/s; 0 für unbegrenzt, live angewendet)',
    bufferBudgetBytes: 'Pufferbudget im Flug (Bytes)',
    bufferBudgetHint:
      'Maximaler RAM für parallele SDK-Multipart-Puffer (z.B. 268435456 für 256 MiB, live angewendet)',
    tuneTitle: 'Replikationsdurchsatz',
    tuneDesc:
      'Steuert, wie viele Kopien parallel laufen und wie Multipart-Uploads aufgeteilt werden. Ein leeres Feld (oder 0) verwendet den im Feld angezeigten Standard.',
    needsRestart: 'Worker-Neustart erforderlich',
    appliesLive: 'Wirkt sofort',
    bulkConcurrencyPercent: 'Anteil Bulk-Parallelität (%)',
    bulkConcurrencyPercentHint:
      'Anteil der Task-Parallelität, der für Bulk-Migration reserviert ist. Standard 20; erlaubt 5 bis 95.',
    multipartThreads: 'Multipart-Threads',
    multipartThreadsHint:
      'Parallele Teil-Uploads je Objekt. Standard 1; erlaubt 1 bis 16.',
    multipartPartSizeMiB: 'Multipart-Teilgröße (MiB)',
    multipartPartSizeMiBHint:
      'Größe jedes hochgeladenen Teils. Standard 16; erlaubt 5 bis 512.',
    invalidBulkPercent:
      '0 (Standard) oder eine ganze Zahl von 5 bis 95 eingeben.',
    invalidMultipartThreads:
      '0 (Standard) oder eine ganze Zahl von 1 bis 16 eingeben.',
    invalidPartSize:
      '0 (Standard) oder eine ganze Zahl von 5 bis 512 eingeben.',
    recTitle: 'Empfohlene Werte',
    recIntro:
      'Startwerte für Ihre Hardware. Nichts wird übernommen, bis Sie auf Übernehmen klicken und speichern.',
    recCores: 'CPU-Kerne je Worker',
    recRam: 'RAM je Worker (GB)',
    recFill: 'Übernehmen',
    recConcurrency: 'Task-Parallelität: etwa 16 je Kern, höchstens 256.',
    recBuffer: 'Pufferbudget: 25 % des RAM, höchstens 8 GiB.',
    recThreads: 'Multipart-Threads: 4 für Objektspeicher über WAN.',
    recBulk: 'Bulk-Anteil: 80 %, wenn der Live-Proxy-Verkehr gering ist.',
    throughputLink: 'Durchsatz-Panel öffnen',
    lockTitle: 'Verteilte Sperren',
    lockDesc: 'Pufferzeit für verteilte Redis-Sperren konfigurieren',
    lockOverlap: 'Sperrüberlappungsdauer',
    lockOverlapHint:
      'Pufferzeit vor Ablauf der Sperre zur Vermeidung von Race Conditions',
    webhookTitle: 'API-Webhook-Integration',
    webhookDesc:
      'Endpunkt für eingehende S3- oder Swift-Ereignisbenachrichtigungen',
    webhookEnabled: 'Webhook-Endpunkt aktivieren',
    webhookBaseUrl: 'Webhook-Basis-URL',
    webhookBaseUrlHint:
      'Öffentlich erreichbare Basis-URL für Benachrichtigungs-Push-Endpunkte',
    saveSuccess:
      'Konfiguration gespeichert. Live-Werte werden asynchron verteilt; Parallelität und Intervalle benötigen einen Worker-Neustart. Diese Seite bestätigt die Laufzeitanwendung nicht.',
    conflictError:
      'Die Konfiguration wurde von einem anderen Benutzer geändert. Bitte neu laden.',
    unsavedChanges: 'Ungespeicherte Änderungen',
    never: 'Nie',
  },
  vi: {
    retry: 'Thử lại',
    diffTitle: 'Xem lại thay đổi trước khi lưu',
    diffText: 'Các cài đặt này sẽ được lưu thành phiên bản {version}.',
    diffNoChanges: 'Không có thay đổi.',
    confirmSave: 'Lưu cài đặt',
    cancel: 'Hủy',
    conflictTitle: 'Cài đặt đã bị người khác thay đổi',
    conflictText:
      'Một quản trị viên khác đã lưu phiên bản mới hơn. Hãy tải lại để xem giá trị của họ (bản nháp của bạn sẽ bị bỏ), hoặc ghi đè để lưu bản nháp của bạn lên phiên bản mới.',
    reload: 'Tải lại cài đặt',
    overwrite: 'Ghi đè bằng bản nháp của tôi',
    unsavedBar: 'Có thay đổi chưa lưu',
    changesCount: '{count} thay đổi',
    bannerFailed: 'Không lưu được cài đặt',
    configuration: 'Cấu hình',
    settingsSections: 'Các nhóm cấu hình hệ thống',
    tabFeatures: 'Tính năng',
    tabBulk: 'Bulk migration',
    tabWorker: 'Worker',
    tabLocking: 'Khóa phân tán',
    tabWebhook: 'Webhook',
    restartRequired: 'Cần restart worker',
    saved: 'Đã lưu',
    validationSummary: '{count} trường cần kiểm tra',
    reviewFields: 'Kiểm tra trường lỗi',
    loading: 'Đang tải cấu hình…',
    savedVersion: 'Phiên bản đã lưu',
    effectiveNotice:
      'Cấu hình đã lưu có thể cần thời gian để đến worker. Concurrency và chu kỳ cần khởi động lại worker.',
    bulkTitle: 'Giới hạn bulk migration',
    bulkDesc:
      'Giới hạn băng thông, bộ đệm multipart và backlog theo từng worker và hàng đợi copy.',
    requestFailed: 'Không thực hiện được yêu cầu cấu hình',
    loadFailed:
      'Chưa tải được cấu hình. Hãy thử lại trước khi chỉnh sửa; không áp dụng giá trị mặc định.',
    draftRetained:
      'Bản nháp vẫn được giữ. Tải phiên bản hiện tại trước khi lưu lại.',
    discardTitle: 'Bỏ các thay đổi chưa lưu?',
    discardText:
      'Bản nháp sẽ bị thay thế hoặc bị bỏ lại. Chỉ tiếp tục nếu không cần giữ các chỉnh sửa này.',
    discardConfirm: 'Bỏ thay đổi',
    keepEditing: 'Tiếp tục chỉnh sửa',
    durationHint: 'Định dạng thời lượng: 500ms, 30s hoặc 1m30s.',
    invalidDuration: 'Nhập thời lượng dương, ví dụ 500ms, 30s hoặc 1m30s.',
    invalidConcurrency: 'Nhập số nguyên từ 1 đến 1.000.',
    invalidWatermark: 'Nhập số nguyên từ 1 đến 2.147.483.647.',
    invalidWatermarkOrder: 'Ngưỡng thấp phải nhỏ hơn ngưỡng cao.',
    invalidBytes:
      'Nhập số byte nguyên không âm (tối đa 9.223.372.036.854.775.807).',
    invalidUrl:
      'Nhập URL HTTP(S) tuyệt đối không kèm tài khoản/mật khẩu, hoặc để trống.',
    title: 'Cấu hình hệ thống',
    subtitle: 'Quản lý tính năng đồng bộ và cấu hình worker.',
    lastUpdated: 'Cập nhật lần cuối',
    version: 'Phiên bản',
    save: 'Lưu cấu hình',
    saving: 'Đang lưu...',
    reset: 'Hủy thay đổi',
    refresh: 'Làm mới',
    featuresTitle: 'Tính năng đồng bộ',
    featuresDesc:
      'Chọn metadata của object và bucket cần đồng bộ giữa các storage.',
    tagging: 'Đồng bộ Thẻ (Tagging)',
    taggingDesc:
      'Đồng bộ hóa các tag được gán cho bucket và object giữa các storage',
    acl: 'Danh sách kiểm soát truy cập (ACL)',
    aclDesc: 'Đồng bộ hóa quyền truy cập object và bucket giữa các storage',
    lifecycle: 'Cấu hình vòng đời (Lifecycle)',
    lifecycleDesc:
      'Đồng bộ hóa quy tắc hết hạn và chuyển đổi trạng thái của bucket',
    policy: 'Chính sách Bucket (Bucket Policy)',
    policyDesc:
      'Đồng bộ hóa các tài liệu chính sách JSON giữa các storage thành viên',
    preserveAclGrants: 'Giữ nguyên ACL Grants từ nguồn',
    preserveAclGrantsDesc:
      'Duy trì các quyền ban đầu thay vì ánh xạ sang chủ sở hữu đích',
    versioning: 'Phiên bản đối tượng (Versioning)',
    versioningDesc: 'Đồng bộ hóa đối tượng có phiên bản và delete marker',
    workerTitle: 'Cấu hình worker',
    workerDesc:
      'Điều chỉnh số tác vụ song song và các chu kỳ xử lý. Các thay đổi trong mục này cần khởi động lại worker.',
    concurrency: 'Số tác vụ song song',
    concurrencyHint:
      'Số tác vụ đồng bộ song song tối đa trên mỗi tiến trình worker.',
    shutdownTimeout: 'Thời gian chờ tắt',
    queueUpdateInterval: 'Chu kỳ cập nhật hàng đợi',
    taskCheckInterval: 'Chu kỳ kiểm tra tác vụ',
    delayedTaskCheckInterval: 'Chu kỳ kiểm tra tác vụ hoãn',
    swiftRetryInterval: 'Chu kỳ thử lại Swift',
    pauseRetryInterval: 'Chu kỳ thử lại khi tạm dừng',
    switchRetryInterval: 'Chu kỳ thử lại chuyển đổi',
    backlogHighWatermark: 'Ngưỡng cao (tác vụ)',
    backlogHighWatermarkHint:
      'Tạm dừng listing khi backlog copy đạt ngưỡng này.',
    backlogLowWatermark: 'Ngưỡng thấp (tác vụ)',
    backlogLowWatermarkHint:
      'Tiếp tục listing khi backlog dưới ngưỡng này. Phải nhỏ hơn ngưỡng cao.',
    bandwidthLimitBytes: 'Băng thông copy (bytes/s)',
    bandwidthLimitHint: 'Theo worker. 0 = không giới hạn; 2097152 = 2 MiB/s.',
    bufferBudgetBytes: 'Ngân sách bộ đệm multipart (bytes)',
    bufferBudgetHint: 'Theo worker. 0 = không giới hạn; 268435456 = 256 MiB.',
    tuneTitle: 'Thông lượng đồng bộ',
    tuneDesc:
      'Điều chỉnh số bản sao chạy song song và cách chia multipart upload. Để trống (hoặc 0) để dùng giá trị mặc định hiển thị trong ô.',
    needsRestart: 'Cần restart worker',
    appliesLive: 'Áp dụng ngay',
    bulkConcurrencyPercent: 'Tỷ lệ song song cho bulk (%)',
    bulkConcurrencyPercentHint:
      'Phần số tác vụ song song dành cho di chuyển hàng loạt. Mặc định 20; cho phép 5 đến 95.',
    multipartThreads: 'Số luồng multipart',
    multipartThreadsHint:
      'Số phần được tải lên song song cho mỗi đối tượng. Mặc định 1; cho phép 1 đến 16.',
    multipartPartSizeMiB: 'Kích thước phần multipart (MiB)',
    multipartPartSizeMiBHint:
      'Kích thước mỗi phần tải lên. Mặc định 16; cho phép 5 đến 512.',
    invalidBulkPercent: 'Nhập 0 (mặc định) hoặc số nguyên từ 5 đến 95.',
    invalidMultipartThreads: 'Nhập 0 (mặc định) hoặc số nguyên từ 1 đến 16.',
    invalidPartSize: 'Nhập 0 (mặc định) hoặc số nguyên từ 5 đến 512.',
    recTitle: 'Giá trị khuyến nghị',
    recIntro:
      'Điểm khởi đầu theo phần cứng của bạn. Không có gì được áp dụng cho đến khi bạn bấm Điền và lưu.',
    recCores: 'Số lõi CPU mỗi worker',
    recRam: 'RAM mỗi worker (GB)',
    recFill: 'Điền',
    recConcurrency: 'Số tác vụ song song: khoảng 16 mỗi lõi, tối đa 256.',
    recBuffer: 'Ngân sách bộ đệm: 25% RAM, tối đa 8 GiB.',
    recThreads: 'Số luồng multipart: 4 cho object storage qua WAN.',
    recBulk: 'Tỷ lệ bulk: 80% khi lưu lượng proxy trực tiếp thấp.',
    throughputLink: 'Mở bảng thông lượng',
    lockTitle: 'Khóa phân tán',
    lockDesc: 'Điều chỉnh khoảng thời gian gối đầu cho khóa giữa các worker.',
    lockOverlap: 'Thời gian gối đầu khóa',
    lockOverlapHint: 'Khoảng đệm thời gian của khóa, ví dụ 2s.',
    webhookTitle: 'Cấu hình webhook',
    webhookDesc: 'Nhận thông báo object từ storage S3 và Swift.',
    webhookEnabled: 'Bật endpoint webhook',
    webhookBaseUrl: 'URL gốc của webhook',
    webhookBaseUrlHint:
      'Địa chỉ HTTP(S) công khai để nhận thông báo từ storage.',
    saveSuccess:
      'Đã lưu cấu hình. Tham số động được truyền bất đồng bộ; concurrency và chu kỳ cần khởi động lại worker. Trang này chưa xác nhận cấu hình đã có hiệu lực tại runtime.',
    conflictError:
      'Cấu hình đã bị thay đổi bởi người quản trị khác. Vui lòng tải lại trang.',
    unsavedChanges: 'Thay đổi chưa lưu',
    never: 'Chưa có',
  },
};
