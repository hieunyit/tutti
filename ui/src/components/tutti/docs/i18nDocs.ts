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

// Plain locale codes keep this file free of the @clyso/clyso-ui-kit bundle.
export default <Record<string, Record<string, string>>>{
  en: {
    title: 'Documentation',
    intro:
      'Tutti puts one S3 endpoint in front of your storages and copies buckets between them.',
    stepsTitle: 'Get started',
    step1Title: 'Add two storages',
    step1Text:
      'Add the storages to keep in sync. Each one needs a connection account that can list, read and write its buckets.',
    step1Link: 'Open Storages',
    step2Title: 'Create a replication',
    step2Text:
      'Choose a source and a destination, then the user and buckets. The initial copy starts right away.',
    step2Link: 'Create replication',
    step3Title: 'Watch progress',
    step3Text:
      'Replication shows the initial copy, lag and errors for every pair of storages.',
    step3Link: 'Open Replication',
    helpTitle: 'Deployment and operations',
    helpText:
      'Guides for Docker Compose, Helm, configuration and upgrades live in the repository.',
    apiLink: 'Browse the API reference',
    'guide_deployment-guide': 'Deployment (Docker Compose, Helm, VM)',
    guide_operations: 'Operations',
    guide_configuration: 'Configuration',
    'guide_storage-accounts': 'Storage accounts',
    'guide_resilience-api': 'Resilience API and CLI',
    'guide_upgrading-from-chorus': 'Upgrading from Chorus',
    'guide_enterprise-ui': 'Enterprise UI',
    guide_failover: 'Failover',
    apiTitle: 'API reference',
    apiSpecMeta: 'OpenAPI 2.0 · {prefix}',
    downloadSpec: 'Download spec',
    apiHint:
      'Try requests in place with your current session. Admin-only calls are marked.',
    apiLoading: 'Loading API reference…',
    apiFailed: 'The API reference could not be loaded.',
    apiNotBundledTitle: 'The API spec is not part of this build',
    apiNotBundledText:
      'This UI was built without the OpenAPI spec (proto/gen/openapi/tutti/tutti.swagger.json).',
    retry: 'Retry',
    roleAdmin: 'admin only',
    roleViewer: 'viewer',
    rolePublic: 'public',
  },
  de: {
    title: 'Dokumentation',
    intro:
      'Tutti stellt einen S3-Endpunkt vor Ihre Speicher und kopiert Buckets zwischen ihnen.',
    stepsTitle: 'Erste Schritte',
    step1Title: 'Zwei Speicher hinzufügen',
    step1Text:
      'Fügen Sie die Speicher hinzu, die synchron bleiben sollen. Jeder braucht ein Verbindungskonto, das Buckets auflisten, lesen und schreiben darf.',
    step1Link: 'Speicher öffnen',
    step2Title: 'Replikation anlegen',
    step2Text:
      'Wählen Sie Quelle und Ziel, dann Benutzer und Buckets. Die Erstkopie startet sofort.',
    step2Link: 'Replikation anlegen',
    step3Title: 'Fortschritt verfolgen',
    step3Text:
      'Replikation zeigt Erstkopie, Verzug und Fehler für jedes Speicherpaar.',
    step3Link: 'Replikation öffnen',
    helpTitle: 'Betrieb und Bereitstellung',
    helpText:
      'Anleitungen für Docker Compose, Helm, Konfiguration und Upgrades liegen im Repository.',
    apiLink: 'API-Referenz ansehen',
    'guide_deployment-guide': 'Bereitstellung (Docker Compose, Helm, VM)',
    guide_operations: 'Betrieb',
    guide_configuration: 'Konfiguration',
    'guide_storage-accounts': 'Speicherkonten',
    'guide_resilience-api': 'Resilience-API und CLI',
    'guide_upgrading-from-chorus': 'Upgrade von Chorus',
    'guide_enterprise-ui': 'Enterprise-Oberfläche',
    guide_failover: 'Failover',
    apiTitle: 'API-Referenz',
    apiSpecMeta: 'OpenAPI 2.0 · {prefix}',
    downloadSpec: 'Spezifikation herunterladen',
    apiHint:
      'Anfragen direkt mit Ihrer aktuellen Sitzung ausprobieren. Nur-Admin-Aufrufe sind markiert.',
    apiLoading: 'API-Referenz wird geladen…',
    apiFailed: 'Die API-Referenz konnte nicht geladen werden.',
    apiNotBundledTitle: 'Die API-Spezifikation ist nicht Teil dieses Builds',
    apiNotBundledText:
      'Diese Oberfläche wurde ohne die OpenAPI-Spezifikation gebaut (proto/gen/openapi/tutti/tutti.swagger.json).',
    retry: 'Erneut versuchen',
    roleAdmin: 'nur Admin',
    roleViewer: 'Viewer',
    rolePublic: 'öffentlich',
  },
  vi: {
    title: 'Tài liệu',
    intro:
      'Tutti đặt một endpoint S3 phía trước các Storage của bạn và sao chép bucket giữa chúng.',
    stepsTitle: 'Bắt đầu',
    step1Title: 'Thêm hai Storage',
    step1Text:
      'Thêm các Storage cần giữ đồng bộ. Mỗi Storage cần một tài khoản kết nối có quyền liệt kê, đọc và ghi bucket.',
    step1Link: 'Mở Storages',
    step2Title: 'Tạo Replication',
    step2Text:
      'Chọn Nguồn và Đích, sau đó chọn User và bucket. Sao chép ban đầu bắt đầu ngay.',
    step2Link: 'Tạo Replication',
    step3Title: 'Theo dõi tiến độ',
    step3Text:
      'Replication hiển thị sao chép ban đầu, độ trễ và lỗi của từng cặp Storage.',
    step3Link: 'Mở Replication',
    helpTitle: 'Triển khai và vận hành',
    helpText:
      'Hướng dẫn Docker Compose, Helm, cấu hình và nâng cấp nằm trong repository.',
    apiLink: 'Xem tài liệu API',
    'guide_deployment-guide': 'Triển khai (Docker Compose, Helm, VM)',
    guide_operations: 'Vận hành',
    guide_configuration: 'Cấu hình',
    'guide_storage-accounts': 'Tài khoản Storage',
    'guide_resilience-api': 'Resilience API và CLI',
    'guide_upgrading-from-chorus': 'Nâng cấp từ Chorus',
    'guide_enterprise-ui': 'Enterprise UI',
    guide_failover: 'Failover',
    apiTitle: 'Tài liệu API',
    apiSpecMeta: 'OpenAPI 2.0 · {prefix}',
    downloadSpec: 'Tải OpenAPI spec',
    apiHint:
      'Thử gọi API ngay với phiên đăng nhập hiện tại. API chỉ dành cho quản trị viên được gắn nhãn.',
    apiLoading: 'Đang tải tài liệu API…',
    apiFailed: 'Không thể tải tài liệu API.',
    apiNotBundledTitle: 'OpenAPI spec không có trong bản build này',
    apiNotBundledText:
      'Giao diện này được dựng mà không đính kèm OpenAPI spec (proto/gen/openapi/tutti/tutti.swagger.json).',
    retry: 'Thử lại',
    roleAdmin: 'Chỉ Admin',
    roleViewer: 'Người xem',
    rolePublic: 'Công khai',
  },
};
