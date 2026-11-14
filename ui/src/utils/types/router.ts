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

export enum RouteName {
  LOGIN = 'LOGIN',
  /** /dashboard: Home + Monitoring (tabs overview, metrics) */
  TUTTI_DASHBOARD = 'TUTTI_DASHBOARD',
  /** /home: redirects to TUTTI_DASHBOARD */
  TUTTI_HOME = 'TUTTI_HOME',
  TUTTI_REPLICATION = 'TUTTI_REPLICATION',
  TUTTI_ADD_REPLICATION = 'TUTTI_ADD_REPLICATION',
  TUTTI_STORAGES = 'TUTTI_STORAGES',
  TUTTI_STORAGE_DETAILS = 'TUTTI_STORAGE_DETAILS',
  TUTTI_SET_CREDENTIAL = 'TUTTI_SET_CREDENTIAL',
  /** /monitoring: redirects to TUTTI_DASHBOARD ?tab=metrics */
  TUTTI_MONITORING = 'TUTTI_MONITORING',
  /** /routing-policies: redirects to TUTTI_REPLICATION ?tab=routing */
  TUTTI_ROUTING_POLICIES = 'TUTTI_ROUTING_POLICIES',
  TUTTI_ADD_ROUTING_POLICY = 'TUTTI_ADD_ROUTING_POLICY',
  /** /diff-reports: redirects to TUTTI_REPLICATION ?tab=diff */
  TUTTI_DIFF_REPORTS = 'TUTTI_DIFF_REPORTS',
  TUTTI_ADD_DIFF_REPORT = 'TUTTI_ADD_DIFF_REPORT',
  /** /diff-reports/detail: redirects to TUTTI_REPLICATION_DIFF_DETAIL */
  TUTTI_DIFF_REPORT_DETAIL = 'TUTTI_DIFF_REPORT_DETAIL',
  /** /replication/diff/:id */
  TUTTI_REPLICATION_DIFF_DETAIL = 'TUTTI_REPLICATION_DIFF_DETAIL',
  TUTTI_PROXIES = 'TUTTI_PROXIES',
  TUTTI_USERS = 'TUTTI_USERS',
  TUTTI_USER_CREATE = 'TUTTI_USER_CREATE',
  TUTTI_USER_DETAIL = 'TUTTI_USER_DETAIL',
  TUTTI_CONSOLE_USERS = 'TUTTI_CONSOLE_USERS',
  TUTTI_CHANGE_PASSWORD = 'TUTTI_CHANGE_PASSWORD',
  TUTTI_FAILOVER = 'TUTTI_FAILOVER',
  TUTTI_PROTECTION_GROUPS = 'TUTTI_PROTECTION_GROUPS',
  TUTTI_PROTECTION_GROUP_DETAIL = 'TUTTI_PROTECTION_GROUP_DETAIL',
  TUTTI_PROXY_ACCOUNTS = 'TUTTI_PROXY_ACCOUNTS',
  TUTTI_SYSTEM_SETTINGS = 'TUTTI_SYSTEM_SETTINGS',
  TUTTI_AUDIT_LOG = 'TUTTI_AUDIT_LOG',
  TUTTI_DOCS = 'TUTTI_DOCS',
  TUTTI_API_REFERENCE = 'TUTTI_API_REFERENCE',
}

export type NavGroupId =
  | 'overview'
  | 'storage'
  | 'dataProtection'
  | 'access'
  | 'administration'
  // not a sidebar group any more (docs open from the topbar Help menu);
  // still the breadcrumb root of /docs and /docs/api
  | 'help';

declare module 'vue-router' {
  interface RouteMeta {
    /** Route performs mutations: blocked for read-only (viewer) accounts. */
    requiresWrite?: boolean;
    /** Administrators only (console users). */
    requiresAdmin?: boolean;
    /** Navigation group (sidebar section + first breadcrumb). */
    navGroup?: NavGroupId;
    /** Sidebar item (route name) this page belongs to; also the 2nd breadcrumb. */
    navSection?: RouteName;
  }
}
