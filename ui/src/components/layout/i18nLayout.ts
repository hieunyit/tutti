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
    skipToContent: 'Skip to main content',
    navLabel: 'Main navigation',
    'group.overview': 'Overview',
    'group.connect': 'Connect',
    'group.replicate': 'Replicate',
    'group.access': 'Access',
    'group.admin': 'Admin',
    'group.help': 'Help',
    consoleTagline: 'S3 replication console',
    badgeLabel: 'open items',
    searchLabel: 'Jump to page',
    searchPlaceholder: 'Jump to page',
    searchResults: 'Pages',
    searchNoResults: 'No matching page',
    themeLabel: 'Theme',
    themeOptionLight: 'Light',
    themeOptionDark: 'Dark',
    themeOptionSystem: 'System',
    help: 'Help',
    helpDocs: 'Documentation',
    helpApi: 'API reference',
    topbarLabel: 'Top bar',
    languageLabel: 'Language',
    authDisabledBanner:
      'Sign-in is disabled: anyone who can reach this console has admin access. Enable api.auth in the worker config.',
    authDisabledLink: 'Read the documentation',
    webhookUnauthenticated:
      'Webhooks are reachable without a token on the main API port. Set api.webhook.token.',
    openMenu: 'Open menu',
    collapse: 'Collapse sidebar',
    expand: 'Expand sidebar',
    breadcrumbs: 'Breadcrumbs',
    about: 'About',
    version: 'Version',
    sourceCode: 'Source code on GitHub',
    close: 'Close',
  },
  [I18nLocale.DE]: {
    skipToContent: 'Zum Hauptinhalt springen',
    navLabel: 'Hauptnavigation',
    'group.overview': 'Übersicht',
    'group.connect': 'Verbinden',
    'group.replicate': 'Replizieren',
    'group.access': 'Zugriff',
    'group.admin': 'Admin',
    'group.help': 'Hilfe',
    consoleTagline: 'S3-Replikationskonsole',
    badgeLabel: 'offene Einträge',
    searchLabel: 'Zu Seite springen',
    searchPlaceholder: 'Zu Seite springen',
    searchResults: 'Seiten',
    searchNoResults: 'Keine passende Seite',
    themeLabel: 'Design',
    themeOptionLight: 'Hell',
    themeOptionDark: 'Dunkel',
    themeOptionSystem: 'System',
    help: 'Hilfe',
    helpDocs: 'Dokumentation',
    helpApi: 'API-Referenz',
    topbarLabel: 'Kopfleiste',
    languageLabel: 'Sprache',
    authDisabledBanner:
      'Die Anmeldung ist deaktiviert: Jeder, der diese Konsole erreichen kann, hat Administratorzugriff. Aktivieren Sie api.auth in der Worker-Konfiguration.',
    authDisabledLink: 'Dokumentation lesen',
    webhookUnauthenticated:
      'Webhooks sind auf dem Haupt-API-Port ohne Token erreichbar. Setzen Sie api.webhook.token.',
    openMenu: 'Menü öffnen',
    collapse: 'Seitenleiste einklappen',
    expand: 'Seitenleiste ausklappen',
    breadcrumbs: 'Brotkrumennavigation',
    about: 'Info',
    version: 'Version',
    sourceCode: 'Quellcode auf GitHub',
    close: 'Schließen',
  },
  vi: {
    skipToContent: 'Chuyển đến nội dung chính',
    navLabel: 'Điều hướng chính',
    'group.overview': 'Tổng quan',
    'group.connect': 'Kết nối',
    'group.replicate': 'Sao chép',
    'group.access': 'Truy cập',
    'group.admin': 'Quản trị',
    'group.help': 'Trợ giúp',
    consoleTagline: 'Bảng điều khiển sao chép S3',
    badgeLabel: 'mục cần xử lý',
    searchLabel: 'Đi tới trang',
    searchPlaceholder: 'Đi tới trang',
    searchResults: 'Trang',
    searchNoResults: 'Không có trang phù hợp',
    themeLabel: 'Giao diện',
    themeOptionLight: 'Sáng',
    themeOptionDark: 'Tối',
    themeOptionSystem: 'Theo hệ thống',
    help: 'Trợ giúp',
    helpDocs: 'Tài liệu',
    helpApi: 'Tham chiếu API',
    topbarLabel: 'Thanh trên',
    languageLabel: 'Ngôn ngữ',
    authDisabledBanner:
      'Đăng nhập đang tắt: bất kỳ ai truy cập được console này đều có quyền admin. Bật api.auth trong cấu hình worker.',
    authDisabledLink: 'Đọc tài liệu',
    webhookUnauthenticated:
      'Webhook truy cập được mà không cần token trên cổng API chính. Hãy đặt api.webhook.token.',
    openMenu: 'Mở menu',
    collapse: 'Thu gọn thanh bên',
    expand: 'Mở rộng thanh bên',
    breadcrumbs: 'Đường dẫn',
    about: 'Giới thiệu',
    version: 'Phiên bản',
    sourceCode: 'Mã nguồn trên GitHub',
    close: 'Đóng',
  },
};
