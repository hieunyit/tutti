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

import type { LocationQuery, RouteLocation, RouteRecordRaw } from 'vue-router';
import { AUDIT_LOG_ROUTE_NAME } from '@/utils/types/audit';
import { RouteName } from '@/utils/types/router';
// WP1: swap this single line for the new Dashboard page (spec 4.1); until
// then /dashboard renders the existing home page.
const TuttiDashboardPage = () =>
  import('@/pages/TuttiHomePage/TuttiHomePage.vue');
const DashboardView = () =>
  import('@/components/dashboard/DashboardView/DashboardView.vue');
const TuttiStoragesPage = () =>
  import('@/pages/TuttiStoragesPage/TuttiStoragesPage.vue');
const ChorusReplicationPage = () =>
  import('@/pages/TuttiReplicationPage/TuttiReplicationPage.vue');
const ChorusAddReplicationPage = () =>
  import('@/pages/TuttiAddReplicationPage/TuttiAddReplicationPage.vue');
const TuttiStorageDetailPage = () =>
  import('@/pages/TuttiStorageDetailsPage/TuttiStorageDetailsPage.vue');
const ChorusAddRoutingPolicyPage = () =>
  import('@/pages/TuttiAddRoutingPolicyPage/TuttiAddRoutingPolicyPage.vue');
const ChorusAddDiffReportPage = () =>
  import('@/pages/TuttiAddDiffReportPage/TuttiAddDiffReportPage.vue');
const ChorusDiffReportDetailPage = () =>
  import('@/pages/TuttiDiffReportDetailPage/TuttiDiffReportDetailPage.vue');
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
const TuttiProtectionGroupsPage = () =>
  import('@/pages/TuttiProtectionGroupsPage/TuttiProtectionGroupsPage.vue');
const TuttiProtectionGroupDetailPage = () =>
  import('@/pages/TuttiProtectionGroupDetailPage/TuttiProtectionGroupDetailPage.vue');
const TuttiProxyAccountsPage = () =>
  import('@/pages/TuttiProxyAccountsPage/TuttiProxyAccountsPage.vue');
const TuttiAuditLogPage = () =>
  import('@/pages/TuttiAuditLogPage/TuttiAuditLogPage.vue');
const TuttiSystemSettingsPage = () =>
  import('@/pages/TuttiSystemSettingsPage/TuttiSystemSettingsPage.vue');
const DocsPage = () => import('@/pages/DocsPage.vue');
const LoginPage = () => import('@/pages/LoginPage/LoginPage.vue');

/** Old URL -> new route, keeping the query; `extra` is merged on top. */
function keepQuery(name: RouteName, extra: LocationQuery = {}) {
  return (to: RouteLocation) => ({
    name,
    query: { ...to.query, ...extra },
    hash: to.hash,
  });
}

/**
 * /diff-reports/detail?from=..&to=.. (and ?id=..) -> /replication/diff/:id.
 * The whole query is kept (the detail store still reads from/to); the id is
 * ?id when given, else "<from>~<to>", which identifies a diff report.
 */
function diffDetailRedirect(to: RouteLocation) {
  const { id, from, to: target } = to.query;
  let detailId = 'report';

  if (typeof id === 'string' && id !== '') detailId = id;
  else if (typeof from === 'string' && typeof target === 'string')
    detailId = `${from}~${target}`;

  return {
    name: RouteName.TUTTI_REPLICATION_DIFF_DETAIL,
    params: { id: detailId },
    query: to.query,
    hash: to.hash,
  };
}

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
    redirect: keepQuery(RouteName.TUTTI_DASHBOARD),
    children: [
      {
        path: 'dashboard',
        meta: { navGroup: 'overview', navSection: RouteName.TUTTI_DASHBOARD },
        name: RouteName.TUTTI_DASHBOARD,
        component: TuttiDashboardPage,
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
        path: 'replication/routing/add',
        meta: {
          requiresWrite: true,
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_REPLICATION,
        },
        name: RouteName.CHORUS_ADD_ROUTING_POLICY,
        component: ChorusAddRoutingPolicyPage,
      },
      {
        path: 'replication/diff/add',
        meta: {
          requiresWrite: true,
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_REPLICATION,
        },
        name: RouteName.CHORUS_ADD_DIFF_REPORT,
        component: ChorusAddDiffReportPage,
      },
      {
        path: 'replication/diff/:id',
        meta: {
          navGroup: 'dataProtection',
          navSection: RouteName.CHORUS_REPLICATION,
        },
        name: RouteName.TUTTI_REPLICATION_DIFF_DETAIL,
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
          navGroup: 'access',
          navSection: RouteName.TUTTI_CONSOLE_USERS,
        },
        name: RouteName.TUTTI_CONSOLE_USERS,
        component: TuttiConsoleUsersPage,
      },
      {
        path: 'audit-log',
        meta: {
          requiresAdmin: true,
          navGroup: 'administration',
          navSection: RouteName.TUTTI_AUDIT_LOG,
        },
        name: AUDIT_LOG_ROUTE_NAME,
        component: TuttiAuditLogPage,
      },
      {
        path: 'system-settings',
        meta: {
          requiresAdmin: true,
          navGroup: 'administration',
          navSection: RouteName.TUTTI_SYSTEM_SETTINGS,
        },
        name: RouteName.TUTTI_SYSTEM_SETTINGS,
        component: TuttiSystemSettingsPage,
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
        path: 'docs/api',
        meta: { navGroup: 'help', navSection: RouteName.TUTTI_API_REFERENCE },
        name: RouteName.TUTTI_API_REFERENCE,
        component: DocsPage,
      },
      {
        path: 'docs/:slug?',
        meta: { navGroup: 'help', navSection: RouteName.TUTTI_DOCS },
        name: RouteName.TUTTI_DOCS,
        component: DocsPage,
      },

      // Old URLs (spec section 2). Each keeps its query string; the old route
      // names stay on the redirect records so `{ name }` links keep working.
      {
        path: 'home',
        name: RouteName.CHORUS_HOME,
        redirect: keepQuery(RouteName.TUTTI_DASHBOARD),
      },
      {
        path: 'monitoring',
        name: RouteName.CHORUS_MONITORING,
        redirect: keepQuery(RouteName.TUTTI_DASHBOARD, { tab: 'metrics' }),
      },
      {
        path: 'health',
        redirect: keepQuery(RouteName.TUTTI_PROXIES),
      },
      {
        path: 'routing-policies',
        name: RouteName.CHORUS_ROUTING_POLICIES,
        redirect: keepQuery(RouteName.CHORUS_REPLICATION, { tab: 'routing' }),
      },
      {
        path: 'routing-policies/add',
        redirect: keepQuery(RouteName.CHORUS_ADD_ROUTING_POLICY),
      },
      {
        path: 'diff-reports',
        name: RouteName.CHORUS_DIFF_REPORTS,
        redirect: keepQuery(RouteName.CHORUS_REPLICATION, { tab: 'diff' }),
      },
      {
        path: 'diff-reports/add',
        redirect: keepQuery(RouteName.CHORUS_ADD_DIFF_REPORT),
      },
      {
        path: 'diff-reports/detail',
        name: RouteName.CHORUS_DIFF_REPORT_DETAIL,
        redirect: diffDetailRedirect,
      },
      {
        path: 'failover',
        redirect: keepQuery(RouteName.TUTTI_PROTECTION_GROUPS),
      },
      {
        path: 'access-keys',
        redirect: keepQuery(RouteName.TUTTI_PROXY_ACCOUNTS),
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
    // a path (not a name) so the unmatched pathMatch param is dropped
    redirect: '/dashboard',
  },
];
