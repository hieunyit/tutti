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
    title: 'Dashboard',
    description:
      'Operations overview: storage health, replication progress and live traffic at a glance.',
    tabOverview: 'Overview',
    tabMetrics: 'Metrics',
    refresh: 'Refresh',
    updatedAt: 'Updated at {time}',
    notUpdated: 'Not updated yet',
    errorText: 'The system status could not be loaded.',
    partialError:
      'Some data could not be loaded and may be outdated: {sources}.',
    sourceStorages: 'storages',
    sourceReplications: 'replications',
    sourceProxies: 'proxy status',
    sourceEvents: 'failover events',
    allGood: 'All systems operational',
    metricsErrorText: 'The traffic history could not be loaded.',
    retry: 'Try again',
  },
  de: {
    title: 'Dashboard',
    description:
      'Betriebsübersicht: Storage-Zustand, Replikationsfortschritt und Live-Traffic auf einen Blick.',
    tabOverview: 'Übersicht',
    tabMetrics: 'Metriken',
    refresh: 'Aktualisieren',
    updatedAt: 'Aktualisiert um {time}',
    notUpdated: 'Noch nicht aktualisiert',
    errorText: 'Der Systemstatus konnte nicht geladen werden.',
    partialError:
      'Einige Daten konnten nicht geladen werden und sind evtl. veraltet: {sources}.',
    sourceStorages: 'Storages',
    sourceReplications: 'Replikationen',
    sourceProxies: 'Proxy-Status',
    sourceEvents: 'Failover-Ereignisse',
    allGood: 'Alle Systeme laufen',
    metricsErrorText: 'Der Traffic-Verlauf konnte nicht geladen werden.',
    retry: 'Erneut versuchen',
  },
  vi: {
    title: 'Dashboard',
    description:
      'Tổng quan vận hành: tình trạng storage, tiến độ replication và lưu lượng trực tiếp.',
    tabOverview: 'Tổng quan',
    tabMetrics: 'Số liệu',
    refresh: 'Làm mới',
    updatedAt: 'Cập nhật lúc {time}',
    notUpdated: 'Chưa cập nhật',
    errorText: 'Không tải được trạng thái hệ thống.',
    partialError: 'Một số dữ liệu không tải được và có thể đã cũ: {sources}.',
    sourceStorages: 'storage',
    sourceReplications: 'replication',
    sourceProxies: 'trạng thái proxy',
    sourceEvents: 'sự kiện failover',
    allGood: 'Mọi hệ thống hoạt động bình thường',
    metricsErrorText: 'Không tải được lịch sử lưu lượng.',
    retry: 'Thử lại',
  },
};
