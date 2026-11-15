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

export default {
  title: 'Audit log',
  retry: 'Retry',
  errorText: 'The audit log could not be loaded. Try again.',
  loadOlder: 'Load older entries',
  loadNewer: 'Load newer entries',
  loadMoreFailed:
    'Could not load more entries. Entries already loaded are kept.',
  allLoaded: 'All entries loaded',
  tableMeta: '{count} entries shown',
  emptyTitle: 'No audit entries yet',
  noMatchTitle: 'No audit entries match',
  clearFilters: 'Clear filters',
  searchLabel: 'Search by object',
  searchPlaceholder: 'e.g. main, alice',
  familyLabel: 'Type',
  familyAll: 'All types',
  actionLabel: 'Action',
  actionAll: 'All actions',
  actorLabel: 'Changed by',
  actorPlaceholder: 'e.g. admin',
  dateRangeLabel: 'Date range',
  limitLabel: 'Entries per load',
  colTimestamp: 'When',
  colActor: 'Changed by',
  colAction: 'Action',
  colObject: 'Object',
  requestId: 'Request ID',
  showDiff: 'View changes',
  drawerTitle: 'Entry #{id}',
  changedFields: 'Changed fields',
  noChangedFields: 'No field-level change detected',
  redactedNote: 'Secrets are masked before an entry is stored.',
  beforeSnapshot: 'Before',
  afterSnapshot: 'After',
  noBeforeSnapshot: 'No earlier state (created)',
  noAfterSnapshot: 'No later state (deleted)',
  redisBannerTitle: 'History is not kept durably',
  redisBannerText:
    'The configuration store is Redis, which does not record audit entries with before and after snapshots. Use PostgreSQL to keep a durable history.',
  readOnlyBannerTitle: 'Configuration is read-only',
  readOnlyBannerText:
    'Changes are blocked while the control plane is read-only. Existing history stays available.',
  family_storage: 'Storage',
  family_s3_identity: 'User',
  family_proxy_account: 'Access key',
  family_access_policy: 'Access policy',
  family_dynamic_credential: 'Dynamic credential',
  family_dynamic_credential_salt: 'Credential salt',
  family_console_user: 'Console user',
  family_console_user_login: 'Console sign-in',
  family_route: 'Routing rule',
  family_route_block: 'Blocked route',
  family_replication: 'Replication',
  family_replication_policy: 'Replication',
  family_replication_status: 'Replication status',
  family_replication_generation: 'Replication generation',
  family_protection_group: 'Protection group',
  family_failover_policy: 'Failover policy',
  family_failover_event: 'Failover event',
  family_diff: 'Diff report',
  family_diff_report: 'Diff report',
  family_storage_test: 'Storage connection test',
  family_proxy_fleet_config: 'Proxy settings',
  family_system_settings: 'System settings',
  action_create: 'Create',
  action_update: 'Update',
  action_delete: 'Delete',
  action_set: 'Set',
  action_rename: 'Rename',
  action_pause: 'Pause',
  action_resume: 'Resume',
  action_start: 'Start',
  action_restart: 'Restart',
  action_fix: 'Fix',
  action_enable: 'Enable',
  action_disable: 'Disable',
  action_block: 'Block',
  action_unblock: 'Unblock',
  action_generation: 'Generation',
  action_archive: 'Archive',
  action_rotate: 'Rotate',
  action_clear_writer: 'Clear writer',
  action_intent: '{action} requested',
};
