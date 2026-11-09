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
import { RouteName } from '@/utils/types/router';
const ChorusHomePage = () =>
  import('@/pages/ChorusHomePage/ChorusHomePage.vue');
const DashboardView = () =>
  import('@/components/dashboard/DashboardView/DashboardView.vue');
const ChorusStoragesPage = () =>
  import('@/pages/ChorusStoragesPage/ChorusStoragesPage.vue');
const ChorusMonitoringPage = () =>
  import('@/pages/ChorusMonitoringPage/ChorusMonitoringPage.vue');
const ChorusReplicationPage = () =>
  import('@/pages/ChorusReplicationPage/ChorusReplicationPage.vue');
const ChorusAddReplicationPage = () =>
  import('@/pages/ChorusAddReplicationPage/ChorusAddReplicationPage.vue');
const ChorusStorageDetailsPage = () =>
  import('@/pages/ChorusStorageDetailsPage/ChorusStorageDetailsPage.vue');
const ChorusRoutingPoliciesPage = () =>
  import('@/pages/ChorusRoutingPoliciesPage/ChorusRoutingPoliciesPage.vue');
const ChorusAddRoutingPolicyPage = () =>
  import('@/pages/ChorusAddRoutingPolicyPage/ChorusAddRoutingPolicyPage.vue');
const ChorusSetCredentialPage = () =>
  import('@/pages/ChorusSetCredentialPage/ChorusSetCredentialPage.vue');
const ChorusDiffReportsPage = () =>
  import('@/pages/ChorusDiffReportsPage/ChorusDiffReportsPage.vue');
const ChorusAddDiffReportPage = () =>
  import('@/pages/ChorusAddDiffReportPage/ChorusAddDiffReportPage.vue');
const ChorusDiffReportDetailPage = () =>
  import('@/pages/ChorusDiffReportDetailPage/ChorusDiffReportDetailPage.vue');
const TuttiHealthPage = () =>
  import('@/pages/TuttiHealthPage/TuttiHealthPage.vue');
const TuttiFailoverPage = () =>
  import('@/pages/TuttiFailoverPage/TuttiFailoverPage.vue');
const TuttiProxyAccountsPage = () =>
  import('@/pages/TuttiProxyAccountsPage/TuttiProxyAccountsPage.vue');
const LoginPage = () => import('@/pages/LoginPage/LoginPage.vue');

export const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: RouteName.LOGIN,
    component: LoginPage,
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
        component: ChorusStoragesPage,
      },
      {
        path: 'storages/:storageName',
        meta: { navGroup: 'storage', navSection: RouteName.CHORUS_STORAGES },
        name: RouteName.CHORUS_STORAGE_DETAILS,
        component: ChorusStorageDetailsPage,
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
        meta: { navGroup: 'storage', navSection: RouteName.TUTTI_HEALTH },
        name: RouteName.TUTTI_HEALTH,
        component: TuttiHealthPage,
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
        path: 'storages/:storageName/credentials/:alias?',
        meta: {
          requiresWrite: true,
          navGroup: 'storage',
          navSection: RouteName.CHORUS_STORAGES,
        },
        name: RouteName.CHORUS_SET_CREDENTIAL,
        component: ChorusSetCredentialPage,
        props: true,
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: { name: RouteName.CHORUS_HOME },
  },
];
