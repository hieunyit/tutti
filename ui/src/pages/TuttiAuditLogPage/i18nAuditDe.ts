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
  title: 'Audit-Protokoll',
  retry: 'Erneut versuchen',
  errorText:
    'Das Audit-Protokoll konnte nicht geladen werden. Versuchen Sie es erneut.',
  loadOlder: 'Mehr laden',
  loadNewer: 'Neuere Einträge laden',
  loadMoreFailed:
    'Weitere Einträge konnten nicht geladen werden. Bereits geladene Einträge bleiben erhalten.',
  allLoaded: 'Alle Einträge geladen',
  tableMeta: '{count} Einträge angezeigt',
  emptyTitle: 'Noch keine Audit-Einträge',
  noMatchTitle: 'Keine Audit-Einträge gefunden',
  clearFilters: 'Filter zurücksetzen',
  searchLabel: 'Nach Objekt suchen',
  searchPlaceholder: 'z. B. main, alice',
  familyLabel: 'Typ',
  familyAll: 'Alle Typen',
  actionLabel: 'Aktion',
  actionAll: 'Alle Aktionen',
  actorLabel: 'Geändert von',
  actorPlaceholder: 'z. B. admin',
  dateRangeLabel: 'Zeitraum',
  limitLabel: 'Einträge pro Abruf',
  colTimestamp: 'Zeitpunkt',
  colActor: 'Geändert von',
  colAction: 'Aktion',
  colObject: 'Objekt',
  requestId: 'Anfrage-ID',
  showDiff: 'Änderungen ansehen',
  drawerTitle: 'Eintrag #{id}',
  changedFields: 'Geänderte Felder',
  noChangedFields: 'Keine Änderung auf Feldebene erkannt',
  redactedNote: 'Geheimnisse werden vor dem Speichern maskiert.',
  beforeSnapshot: 'Vorher',
  afterSnapshot: 'Nachher',
  noBeforeSnapshot: 'Kein früherer Zustand erfasst',
  noAfterSnapshot: 'Kein späterer Zustand erfasst',
  redisBannerTitle: 'Der Verlauf wird nicht dauerhaft gespeichert',
  redisBannerText:
    'Der Konfigurationsspeicher ist Redis und zeichnet keine Audit-Einträge mit Vorher-Nachher-Stand auf. Mit PostgreSQL bleibt der Verlauf dauerhaft erhalten.',
  readOnlyBannerTitle: 'Konfiguration ist schreibgeschützt',
  readOnlyBannerText:
    'Änderungen sind gesperrt, solange die Control Plane schreibgeschützt ist. Der bisherige Verlauf bleibt verfügbar.',
  family_storage: 'Speicher',
  family_s3_identity: 'Benutzer',
  family_proxy_account: 'Zugriffsschlüssel',
  family_access_policy: 'Zugriffsrichtlinie',
  family_dynamic_credential: 'Dynamische Zugangsdaten',
  family_dynamic_credential_salt: 'Zugangsdaten-Salt',
  family_console_user: 'Konsolenbenutzer',
  family_console_user_login: 'Konsolenanmeldung',
  family_route: 'Routing-Regel',
  family_route_block: 'Gesperrte Route',
  family_replication: 'Replikation',
  family_replication_policy: 'Replikation',
  family_replication_status: 'Replikationsstatus',
  family_replication_generation: 'Replikationsgeneration',
  family_protection_group: 'Schutzgruppe',
  family_failover_policy: 'Failover-Richtlinie',
  family_failover_event: 'Failover-Ereignis',
  family_diff: 'Diff-Report',
  family_diff_report: 'Diff-Report',
  family_storage_test: 'Speicher-Verbindungstest',
  family_proxy_fleet_config: 'Proxy-Einstellungen',
  family_system_settings: 'Systemeinstellungen',
  action_create: 'Erstellen',
  action_update: 'Aktualisieren',
  action_delete: 'Löschen',
  action_set: 'Setzen',
  action_rename: 'Umbenennen',
  action_pause: 'Pausieren',
  action_resume: 'Fortsetzen',
  action_start: 'Starten',
  action_restart: 'Neu starten',
  action_fix: 'Beheben',
  action_enable: 'Aktivieren',
  action_disable: 'Deaktivieren',
  action_block: 'Sperren',
  action_unblock: 'Entsperren',
  action_generation: 'Generation',
  action_archive: 'Archivieren',
  action_rotate: 'Rotieren',
  action_clear_writer: 'Schreiber freigeben',
  action_intent: '{action} angefordert',
};
