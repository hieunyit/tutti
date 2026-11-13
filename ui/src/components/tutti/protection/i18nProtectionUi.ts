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
import base from './i18nProtection';

// Strings of the redesigned pages (list, wizard, detail tabs). Merged over the
// existing protection messages; the vi locale falls back to en for the rest.
const extra: Record<'en' | 'de' | 'vi', Record<string, string>> = {
  en: {
    kpiSwitching: 'Switching',
    kpiSwitchingSub: 'groups with a switchover in progress',
    kpiMaxLag: 'Max lag',
    kpiMaxLagSub: 'highest event lag of any member',
    freezeSwitch: 'Automatic failover',
    freezeSwitchOn: 'Active',
    freezeSwitchOff: 'Frozen',
    freezeErrorTitle: 'Could not change automatic failover',
    wizTitle: 'Create protection group',
    wizStepScope: 'Scope',
    wizStepMembers: 'Copies and primary',
    wizStepConfirm: 'Confirm',
    wizScopeDesc: 'Name the group and choose the buckets it protects.',
    wizMembersDesc:
      'Members in order: the first storage is the primary. Choose how many copies to keep.',
    wizFailoverTitle: 'Failover',
    wizOptionsTitle: 'Options',
    wizConfirmDesc: 'Review the group, then create it.',
    wizFinish: 'Create group',
    wizAllBuckets: 'All buckets',
    wizPrimary: 'Primary',
    wizSaveFailed: 'The group was not created',
    tabMembers: 'Members',
    tabWriters: 'Writers',
    tabActivity: 'Activity',
    tabSettings: 'Settings',
    statusReady: 'Ready',
    dangerTitle: 'Danger zone',
    dangerDesc: 'Detaching a group stops failover management for it.',
    deleteTypeNote: 'Detach group',
    editTitle: 'Edit settings',
    editDesc: 'Scope and members cannot change after creation.',
    editAction: 'Edit settings',
    actionFailed: 'The action failed',
    refreshDetail: 'Refresh',
  },
  de: {
    kpiSwitching: 'Umschaltung',
    kpiSwitchingSub: 'Gruppen mit laufender Umschaltung',
    kpiMaxLag: 'Max. Verzug',
    kpiMaxLagSub: 'höchster Ereignisverzug eines Mitglieds',
    freezeSwitch: 'Automatisches Failover',
    freezeSwitchOn: 'Aktiv',
    freezeSwitchOff: 'Eingefroren',
    freezeErrorTitle: 'Automatisches Failover konnte nicht geändert werden',
    wizTitle: 'Schutzgruppe erstellen',
    wizStepScope: 'Umfang',
    wizStepMembers: 'Kopien und Primary',
    wizStepConfirm: 'Bestätigen',
    wizScopeDesc: 'Benennen Sie die Gruppe und wählen Sie die Buckets.',
    wizMembersDesc:
      'Mitglieder in Reihenfolge: der erste Speicher ist der Primary. Wählen Sie die Anzahl der Kopien.',
    wizFailoverTitle: 'Failover',
    wizOptionsTitle: 'Optionen',
    wizConfirmDesc: 'Prüfen Sie die Gruppe und erstellen Sie sie.',
    wizFinish: 'Gruppe erstellen',
    wizAllBuckets: 'Alle Buckets',
    wizPrimary: 'Primary',
    wizSaveFailed: 'Die Gruppe wurde nicht erstellt',
    tabMembers: 'Mitglieder',
    tabWriters: 'Writer',
    tabActivity: 'Aktivität',
    tabSettings: 'Einstellungen',
    statusReady: 'Bereit',
    dangerTitle: 'Gefahrenzone',
    dangerDesc: 'Beim Lösen endet die Failover-Verwaltung der Gruppe.',
    deleteTypeNote: 'Gruppe lösen',
    editTitle: 'Einstellungen bearbeiten',
    editDesc: 'Umfang und Mitglieder sind nach dem Erstellen fest.',
    editAction: 'Einstellungen bearbeiten',
    actionFailed: 'Die Aktion ist fehlgeschlagen',
    refreshDetail: 'Aktualisieren',
  },
  vi: {
    kpiSwitching: 'Đang chuyển',
    kpiSwitchingSub: 'nhóm đang switchover',
    kpiMaxLag: 'Lag tối đa',
    kpiMaxLagSub: 'độ trễ sự kiện cao nhất của một member',
    freezeSwitch: 'Failover tự động',
    freezeSwitchOn: 'Đang bật',
    freezeSwitchOff: 'Đã freeze',
    freezeErrorTitle: 'Không đổi được failover tự động',
    wizTitle: 'Tạo nhóm bảo vệ',
    wizStepScope: 'Phạm vi',
    wizStepMembers: 'Bản sao và primary',
    wizStepConfirm: 'Xác nhận',
    wizScopeDesc: 'Đặt tên nhóm và chọn các bucket cần bảo vệ.',
    wizMembersDesc:
      'Member theo thứ tự: storage đầu tiên là primary. Chọn số bản sao cần giữ.',
    wizFailoverTitle: 'Failover',
    wizOptionsTitle: 'Tùy chọn',
    wizConfirmDesc: 'Kiểm tra lại nhóm rồi tạo.',
    wizFinish: 'Tạo nhóm',
    wizAllBuckets: 'Tất cả bucket',
    wizPrimary: 'Primary',
    wizSaveFailed: 'Chưa tạo được nhóm',
    tabMembers: 'Member',
    tabWriters: 'Writer',
    tabActivity: 'Hoạt động',
    tabSettings: 'Cài đặt',
    statusReady: 'Sẵn sàng',
    dangerTitle: 'Vùng nguy hiểm',
    dangerDesc: 'Gỡ nhóm sẽ dừng quản lý failover cho nhóm này.',
    deleteTypeNote: 'Gỡ nhóm',
    editTitle: 'Sửa cài đặt',
    editDesc: 'Không đổi được phạm vi và member sau khi tạo.',
    editAction: 'Sửa cài đặt',
    actionFailed: 'Thao tác thất bại',
    refreshDetail: 'Làm mới',
  },
};

const baseMessages = base as unknown as Record<string, Record<string, unknown>>;

export default <I18nMessages>{
  [I18nLocale.EN]: { ...baseMessages[I18nLocale.EN], ...extra.en },
  [I18nLocale.DE]: { ...baseMessages[I18nLocale.DE], ...extra.de },
  vi: { ...baseMessages.vi, ...extra.vi },
};
