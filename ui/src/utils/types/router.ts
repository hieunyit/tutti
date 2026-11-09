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
  CHORUS_HOME = 'CHORUS_HOME',
  CHORUS_REPLICATION = 'CHORUS_REPLICATION',
  CHORUS_ADD_REPLICATION = 'CHORUS_ADD_REPLICATION',
  CHORUS_STORAGES = 'CHORUS_STORAGES',
  CHORUS_STORAGE_DETAILS = 'CHORUS_STORAGE_DETAILS',
  CHORUS_MONITORING = 'CHORUS_MONITORING',
  CHORUS_ROUTING_POLICIES = 'CHORUS_ROUTING_POLICIES',
  CHORUS_ADD_ROUTING_POLICY = 'CHORUS_ADD_ROUTING_POLICY',
  CHORUS_SET_CREDENTIAL = 'CHORUS_SET_CREDENTIAL',
  CHORUS_DIFF_REPORTS = 'CHORUS_DIFF_REPORTS',
  CHORUS_ADD_DIFF_REPORT = 'CHORUS_ADD_DIFF_REPORT',
  CHORUS_DIFF_REPORT_DETAIL = 'CHORUS_DIFF_REPORT_DETAIL',
  TUTTI_HEALTH = 'TUTTI_HEALTH',
  TUTTI_FAILOVER = 'TUTTI_FAILOVER',
  TUTTI_PROXY_ACCOUNTS = 'TUTTI_PROXY_ACCOUNTS',
  TUTTI_DOCS = 'TUTTI_DOCS',
  TUTTI_API_REFERENCE = 'TUTTI_API_REFERENCE',
}

export type NavGroupId =
  | 'overview'
  | 'storage'
  | 'dataProtection'
  | 'access'
  | 'help';

declare module 'vue-router' {
  interface RouteMeta {
    /** Route performs mutations: blocked for read-only (viewer) accounts. */
    requiresWrite?: boolean;
    /** Navigation group (sidebar section + first breadcrumb). */
    navGroup?: NavGroupId;
    /** Sidebar item (route name) this page belongs to; also the 2nd breadcrumb. */
    navSection?: RouteName;
  }
}
