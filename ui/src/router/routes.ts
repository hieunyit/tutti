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

import type { RouteRecordRaw } from 'vue-router';
import { AUDIT_LOG_ROUTE_NAME } from '@/utils/types/audit';
import { RouteName } from '@/utils/types/router';
const ChorusHomePage = () =>
  import('@/pages/ChorusHomePage/ChorusHomePage.vue');
const DashboardView = () =>
  import('@/components/dashboard/DashboardView/DashboardView.vue');
const TuttiStoragesPage = () =>
  import('@/pages/ChorusStoragesPage/ChorusStoragesPage.vue');
const ChorusMonitoringPage = () =>
  import('@/pages/ChorusMonitoringPage/ChorusMonitoringPage.vue');
const ChorusReplicationPage = () =>
  import('@/pages/ChorusReplicationPage/ChorusReplicationPage.vue');
const ChorusAddReplicationPage = () =>
  import('@/pages/ChorusAddReplicationPage/ChorusAddReplicationPage.vue');
const TuttiStorageDetailPage = () =>
  import('@/pages/ChorusStorageDetailsPage/ChorusStorageDetailsPage.vue');
const ChorusRoutingPoliciesPage = () =>
  import('@/pages/ChorusRoutingPoliciesPage/ChorusRoutingPoliciesPage.vue');
const ChorusAddRoutingPolicyPage = () =>
  import('@/pages/ChorusAddRoutingPolicyPage/ChorusAddRoutingPolicyPage.vue');
const ChorusDiffReportsPage = () =>
  import('@/pages/ChorusDiffReportsPage/ChorusDiffReportsPage.vue');
const ChorusAddDiffReportPage = () =>
  import('@/pages/ChorusAddDiffReportPage/ChorusAddDiffReportPage.vue');
const ChorusDiffReportDetailPage = () =>
  import('@/pages/ChorusDiffReportDetailPage/ChorusDiffReportDetailPage.vue');
const TuttiProxiesPage = () =>
  import('@/pages/TuttiProxiesPage/TuttiProxiesPage.vue');
const TuttiUsersPage = () =>
  import('@/pages/TuttiUsersPage/TuttiUsersPage.vue');
const TuttiUserCreatePage = () =>
  import('@/pages/TuttiUserCreatePage/TuttiUserCreatePage.vue');
const TuttiUserDetailPage = () =>
  import('@/pages/TuttiUserDetailPage/TuttiUserDetailPage.vue');
const TuttiConsoleUsersPage = () =>
  import('@/pages/TuttiConsoleUsersPage/TuttiConsoleUsersPage.vue');
const TuttiChangePasswordPage = () =>
  import('@/pages/TuttiChangePasswordPage/TuttiChangePasswordPage.vue');
const TuttiFailoverPage = () =>
  import('@/pages/TuttiFailoverPage/TuttiFailoverPage.vue');
const TuttiProtectionGroupsPage = () =>
  import('@/pages/TuttiProtectionGroupsPage/TuttiProtectionGroupsPage.vue');
const TuttiProtectionGroupDetailPage = () =>
  import(
    '@/pages/TuttiProtectionGroupDetailPage/TuttiProtectionGroupDetailPage.vue'
  );
const TuttiProxyAccountsPage = () =>
  import('@/pages/TuttiProxyAccountsPage/TuttiProxyAccountsPage.vue');
const TuttiAuditLogPage = () =>
  import('@/pages/TuttiAuditLogPage/TuttiAuditLogPage.vue');
const DocsPage = () => import('@/pages/DocsPage.vue');
const LoginPage = () => import('@/pages/LoginPage/LoginPage.vue');

export const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: RouteName.LOGIN,
    component: LoginPage,
  },
  {
    path: '/change-password',
    name: RouteName.TUTTI_CHANGE_PASSWORD,
    component: TuttiChangePasswordPage,
  },
  {
    path: '/',
    component: DashboardView,
    redirect: { name: RouteName.CHORUS_HOME },
    children: [
      {
        path: 'home',
        meta: { navGroup: 'overview', navSection: RouteName.CHORUS_HOME },
        name: RouteName.CHORUS_HOME,
        component: ChorusHomePage,
      },
      {
        path: 'replication',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_REPLICATION,
        },
        name: RouteName.CHORUS_REPLICATION,
        component: ChorusReplicationPage,
      },
      {
        path: 'replication/add',
        meta: {
          requiresWrite: true,
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_REPLICATION,
        },
        name: RouteName.CHORUS_ADD_REPLICATION,
        component: ChorusAddReplicationPage,
      },
      {
        path: 'diff-reports',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_DIFF_REPORTS,
        },
        name: RouteName.CHORUS_DIFF_REPORTS,
        component: ChorusDiffReportsPage,
      },
      {
        path: 'diff-reports/add',
        meta: {
          requiresWrite: true,
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_DIFF_REPORTS,
        },
        name: RouteName.CHORUS_ADD_DIFF_REPORT,
        component: ChorusAddDiffReportPage,
      },
      {
        path: 'diff-reports/detail',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_DIFF_REPORTS,
        },
        name: RouteName.CHORUS_DIFF_REPORT_DETAIL,
        component: ChorusDiffReportDetailPage,
      },
      {
        path: 'storages',
        meta: { navGroup: 'storage', navSection: RouteName.CHORUS_STORAGES },
        name: RouteName.CHORUS_STORAGES,
        component: TuttiStoragesPage,
      },
      {
        path: 'storages/:storageName',
        meta: { navGroup: 'storage', navSection: RouteName.CHORUS_STORAGES },
        name: RouteName.CHORUS_STORAGE_DETAILS,
        component: TuttiStorageDetailPage,
        props: true,
      },
      {
        path: 'monitoring',
        meta: { navGroup: 'overview', navSection: RouteName.CHORUS_MONITORING },
        name: RouteName.CHORUS_MONITORING,
        component: ChorusMonitoringPage,
      },
      {
        path: 'routing-policies',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_ROUTING_POLICIES,
        },
        name: RouteName.CHORUS_ROUTING_POLICIES,
        component: ChorusRoutingPoliciesPage,
      },
      {
        path: 'routing-policies/add',
        meta: {
          requiresWrite: true,
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_ROUTING_POLICIES,
        },
        name: RouteName.CHORUS_ADD_ROUTING_POLICY,
        component: ChorusAddRoutingPolicyPage,
      },
      {
        path: 'health',
        redirect: { name: RouteName.CHORUS_STORAGES },
      },
      {
        path: 'proxies',
        meta: { navGroup: 'storage', navSection: RouteName.TUTTI_PROXIES },
        name: RouteName.TUTTI_PROXIES,
        component: TuttiProxiesPage,
      },
      {
        path: 'users',
        meta: { navGroup: 'access', navSection: RouteName.TUTTI_USERS },
        name: RouteName.TUTTI_USERS,
        component: TuttiUsersPage,
      },
      {
        path: 'users/new',
        meta: {
          requiresWrite: true,
          navGroup: 'access',
          navSection: RouteName.TUTTI_USERS,
        },
        name: RouteName.TUTTI_USER_CREATE,
        component: TuttiUserCreatePage,
      },
      {
        path: 'users/:name',
        meta: { navGroup: 'access', navSection: RouteName.TUTTI_USERS },
        name: RouteName.TUTTI_USER_DETAIL,
        component: TuttiUserDetailPage,
        props: true,
      },
      {
        path: 'console-users',
        meta: {
          requiresAdmin: true,
          navGroup: 'administration',
          navSection: RouteName.TUTTI_CONSOLE_USERS,
        },
        name: RouteName.TUTTI_CONSOLE_USERS,
        component: TuttiConsoleUsersPage,
      },
      {
        path: 'audit-log',
        meta: {
          navGroup: 'administration',
          navSection: AUDIT_LOG_ROUTE_NAME as unknown as RouteName,
        },
        name: AUDIT_LOG_ROUTE_NAME,
        component: TuttiAuditLogPage,
      },
      {
        path: 'access-keys',
        redirect: { name: RouteName.TUTTI_PROXY_ACCOUNTS },
      },
      {
        path: 'proxy-accounts',
        meta: {
          navGroup: 'access',
          navSection: RouteName.TUTTI_PROXY_ACCOUNTS,
        },
        name: RouteName.TUTTI_PROXY_ACCOUNTS,
        component: TuttiProxyAccountsPage,
      },
      {
        path: 'failover',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.TUTTI_FAILOVER,
        },
        name: RouteName.TUTTI_FAILOVER,
        component: TuttiFailoverPage,
      },
      {
        path: 'protection-groups',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.TUTTI_PROTECTION_GROUPS,
        },
        name: RouteName.TUTTI_PROTECTION_GROUPS,
        component: TuttiProtectionGroupsPage,
      },
      {
        path: 'protection-groups/:name',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.TUTTI_PROTECTION_GROUPS,
        },
        name: RouteName.TUTTI_PROTECTION_GROUP_DETAIL,
        component: TuttiProtectionGroupDetailPage,
        props: true,
      },
      {
        // /docs/api (API reference) is matched first: static segments win
        path: 'docs/:slug?',
        meta: { navGroup: 'help', navSection: RouteName.TUTTI_DOCS },
        name: RouteName.TUTTI_DOCS,
        component: DocsPage,
      },
      {
        path: 'docs/api',
        meta: { navGroup: 'help', navSection: RouteName.TUTTI_API_REFERENCE },
        name: RouteName.TUTTI_API_REFERENCE,
        component: DocsPage,
      },
      {
        // the old credentials page: credentials now live on the user pages
        name: RouteName.CHORUS_SET_CREDENTIAL,
        path: 'storages/:storageName/credentials/:alias?',
        redirect: (to) => {
          const alias = to.params.alias;

          return typeof alias === 'string' && alias !== ''
            ? { name: RouteName.TUTTI_USER_DETAIL, params: { name: alias } }
            : {
                name: RouteName.CHORUS_STORAGE_DETAILS,
                params: { storageName: String(to.params.storageName) },
                query: { tab: 'users' },
              };
        },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: { name: RouteName.CHORUS_HOME },
  },
];
