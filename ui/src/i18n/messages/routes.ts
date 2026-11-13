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

import { type I18nMessages, I18nLocale } from '@clyso/clyso-ui-kit';
import { RouteName } from '@/utils/types/router';
import { AUDIT_LOG_ROUTE_NAME } from '@/utils/types/audit';

type Routes = {
  [k in RouteName]: string;
};

const routes: I18nMessages<Routes> & { vi: Routes } = {
  [I18nLocale.EN]: {
    [RouteName.LOGIN]: 'Login',
    [RouteName.TUTTI_DASHBOARD]: 'Dashboard',
    [RouteName.CHORUS_HOME]: 'Dashboard',
    [RouteName.CHORUS_STORAGES]: 'Storages',
    [RouteName.CHORUS_STORAGE_DETAILS]: 'Storage details',
    [RouteName.CHORUS_SET_CREDENTIAL]: 'Storage credentials',
    [RouteName.CHORUS_REPLICATION]: 'Replication',
    [RouteName.CHORUS_ADD_REPLICATION]: 'Add replication',
    [RouteName.CHORUS_MONITORING]: 'Monitoring',
    [RouteName.CHORUS_ROUTING_POLICIES]: 'Routing policies',
    [RouteName.CHORUS_ADD_ROUTING_POLICY]: 'Add routing policy',
    [RouteName.CHORUS_DIFF_REPORTS]: 'Diff reports',
    [RouteName.CHORUS_ADD_DIFF_REPORT]: 'Add diff report',
    [RouteName.CHORUS_DIFF_REPORT_DETAIL]: 'Diff report detail',
    [RouteName.TUTTI_REPLICATION_DIFF_DETAIL]: 'Diff report',
    [RouteName.TUTTI_PROXIES]: 'Proxies',
    [RouteName.TUTTI_USERS]: 'S3 users',
    [RouteName.TUTTI_USER_CREATE]: 'Create user',
    [RouteName.TUTTI_USER_DETAIL]: 'User details',
    [RouteName.TUTTI_CONSOLE_USERS]: 'Console users',
    [RouteName.TUTTI_CHANGE_PASSWORD]: 'Change password',
    [RouteName.TUTTI_FAILOVER]: 'Failover',
    [RouteName.TUTTI_PROTECTION_GROUPS]: 'Protection groups',
    [RouteName.TUTTI_PROTECTION_GROUP_DETAIL]: 'Protection group',
    [RouteName.TUTTI_PROXY_ACCOUNTS]: 'Proxy accounts',
    [RouteName.TUTTI_SYSTEM_SETTINGS]: 'System settings',
    [AUDIT_LOG_ROUTE_NAME]: 'Audit log',
    [RouteName.TUTTI_DOCS]: 'Documentation',
    [RouteName.TUTTI_API_REFERENCE]: 'API reference',
  },
  [I18nLocale.DE]: {
    [RouteName.LOGIN]: 'Login',
    [RouteName.TUTTI_DASHBOARD]: 'Dashboard',
    [RouteName.CHORUS_HOME]: 'Dashboard',
    [RouteName.CHORUS_STORAGES]: 'Speicher',
    [RouteName.CHORUS_STORAGE_DETAILS]: 'Speicherdetails',
    [RouteName.CHORUS_SET_CREDENTIAL]: 'Speicher-Zugangsdaten',
    [RouteName.CHORUS_REPLICATION]: 'Replikation',
    [RouteName.CHORUS_ADD_REPLICATION]: 'Replikation hinzufügen',
    [RouteName.CHORUS_MONITORING]: 'Monitoring',
    [RouteName.CHORUS_ROUTING_POLICIES]: 'Routing-Richtlinien',
    [RouteName.CHORUS_ADD_ROUTING_POLICY]: 'Routing-Richtlinie hinzufügen',
    [RouteName.CHORUS_DIFF_REPORTS]: 'Diff-Reports',
    [RouteName.CHORUS_ADD_DIFF_REPORT]: 'Diff-Report erstellen',
    [RouteName.CHORUS_DIFF_REPORT_DETAIL]: 'Diff-Report Details',
    [RouteName.TUTTI_REPLICATION_DIFF_DETAIL]: 'Diff-Report',
    [RouteName.TUTTI_PROXIES]: 'Proxys',
    [RouteName.TUTTI_USERS]: 'S3-Benutzer',
    [RouteName.TUTTI_USER_CREATE]: 'Benutzer anlegen',
    [RouteName.TUTTI_USER_DETAIL]: 'Benutzerdetails',
    [RouteName.TUTTI_CONSOLE_USERS]: 'Konsolenbenutzer',
    [RouteName.TUTTI_CHANGE_PASSWORD]: 'Passwort ändern',
    [RouteName.TUTTI_FAILOVER]: 'Failover',
    [RouteName.TUTTI_PROTECTION_GROUPS]: 'Schutzgruppen',
    [RouteName.TUTTI_PROTECTION_GROUP_DETAIL]: 'Schutzgruppe',
    [RouteName.TUTTI_PROXY_ACCOUNTS]: 'Proxy-Konten',
    [RouteName.TUTTI_SYSTEM_SETTINGS]: 'Systemeinstellungen',
    [AUDIT_LOG_ROUTE_NAME]: 'Audit-Protokoll',
    [RouteName.TUTTI_DOCS]: 'Dokumentation',
    [RouteName.TUTTI_API_REFERENCE]: 'API-Referenz',
  },
  vi: {
    [RouteName.LOGIN]: 'Đăng nhập',
    [RouteName.TUTTI_DASHBOARD]: 'Bảng điều khiển',
    [RouteName.CHORUS_HOME]: 'Bảng điều khiển',
    [RouteName.CHORUS_STORAGES]: 'Storages',
    [RouteName.CHORUS_STORAGE_DETAILS]: 'Chi tiết storage',
    [RouteName.CHORUS_SET_CREDENTIAL]: 'Credential của storage',
    [RouteName.CHORUS_REPLICATION]: 'Sao chép',
    [RouteName.CHORUS_ADD_REPLICATION]: 'Thêm replication',
    [RouteName.CHORUS_MONITORING]: 'Giám sát',
    [RouteName.CHORUS_ROUTING_POLICIES]: 'Routing policy',
    [RouteName.CHORUS_ADD_ROUTING_POLICY]: 'Thêm routing policy',
    [RouteName.CHORUS_DIFF_REPORTS]: 'Báo cáo diff',
    [RouteName.CHORUS_ADD_DIFF_REPORT]: 'Thêm báo cáo diff',
    [RouteName.CHORUS_DIFF_REPORT_DETAIL]: 'Chi tiết báo cáo diff',
    [RouteName.TUTTI_REPLICATION_DIFF_DETAIL]: 'Báo cáo diff',
    [RouteName.TUTTI_PROXIES]: 'Proxies',
    [RouteName.TUTTI_USERS]: 'S3 users',
    [RouteName.TUTTI_USER_CREATE]: 'Tạo user',
    [RouteName.TUTTI_USER_DETAIL]: 'Chi tiết user',
    [RouteName.TUTTI_CONSOLE_USERS]: 'Người dùng console',
    [RouteName.TUTTI_CHANGE_PASSWORD]: 'Đổi mật khẩu',
    [RouteName.TUTTI_FAILOVER]: 'Failover',
    [RouteName.TUTTI_PROTECTION_GROUPS]: 'Nhóm bảo vệ',
    [RouteName.TUTTI_PROTECTION_GROUP_DETAIL]: 'Nhóm bảo vệ',
    [RouteName.TUTTI_PROXY_ACCOUNTS]: 'Proxy accounts',
    [RouteName.TUTTI_SYSTEM_SETTINGS]: 'Cấu hình hệ thống',
    [AUDIT_LOG_ROUTE_NAME]: 'Nhật ký kiểm toán',
    [RouteName.TUTTI_DOCS]: 'Tài liệu',
    [RouteName.TUTTI_API_REFERENCE]: 'Tham chiếu API',
  },
};

export default routes;
