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
  title: 'Schutzgruppen',
  retry: 'Erneut versuchen',
  create: 'Schutzgruppe erstellen',
  errorText:
    'Schutzgruppen konnten nicht geladen werden. Prüfen Sie, ob die Funktion aktiviert ist, und versuchen Sie es erneut.',
  freezeHelp:
    'Gilt für alle Gruppen. Eingefroren schaltet keine Gruppe selbst um; manuelles Failover funktioniert weiter.',
  freezeEnableTitle: 'Automatisches Failover einfrieren?',
  freezeEnableText:
    'Keine Gruppe führt ein Failover selbstständig durch, bis Sie fortsetzen. Fällt ein Primary-Speicher aus, bleiben Schreibzugriffe auf dem ausgefallenen Primary und Kopien werden nicht automatisch wiederhergestellt.',
  freezeEnableAction: 'Einfrieren',
  freezeDisableTitle: 'Automatisches Failover fortsetzen?',
  freezeDisableText:
    'Gruppen im automatischen Modus dürfen sofort ein anderes Mitglied befördern, wenn die Mehrheit der Proxys den Primary als ausgefallen meldet.',
  freezeDisableAction: 'Fortsetzen',
  freezeFailed:
    'Der Schalter für automatisches Failover konnte nicht geändert werden',
  emptyTitle: 'Noch keine Schutzgruppen',
  emptyText:
    'Halten Sie Buckets auf mindestens zwei Speichern, damit sie bei einem Ausfall verfügbar bleiben.',
  colName: 'Gruppe',
  colScope: 'Umfasst',
  colCopies: 'Kopien',
  colLag: 'Max. Lag',
  colState: 'Status',
  colMode: 'Modus',
  colChange: 'Letzte Änderung',
  scopeUser: 'Alle Buckets von {user}',
  scopeStorage: 'Alle Buckets auf diesen Speichern',
  scopeBuckets: '{count} Buckets',
  scopeBucketsOne: '1 Bucket',
  primaryMark: 'Primary',
  copiesValue: '{healthy} / {wanted}',
  modeAuto: 'Automatisch',
  modeManual: 'Manuell',
  'state.ready': 'Bereit',
  'state.initializing': 'Initialisierung',
  'state.waiting': 'Wartend',
  'state.draining': 'Leerlaufend',
  'state.promoting': 'Beförderung',
  'state.frozen': 'Eingefroren',
  'state.error': 'Fehler',
  'state.unknown': 'Unbekannt',
  dialogEdit: 'Gruppe {id} bearbeiten',
  fieldId: 'Gruppenname',
  fieldIdHelp:
    'Buchstaben, Ziffern, Bindestrich. Keine Leerzeichen, kein Doppelpunkt oder Schrägstrich.',
  fieldBuckets: 'Buckets (optional)',
  fieldBucketsHelp:
    'Exakte Bucket-Namen ohne Wildcards. Leer lassen, um alle Buckets der gewählten Speicher zu schützen.',
  fieldBucketsPlaceholder: 'Bucket-Namen eingeben und Enter drücken',
  fieldMembers: 'Mitglieder, in Reihenfolge',
  fieldMembersHelp:
    'Der erste Speicher ist der Primary und muss aktuell die Anfragen für diese Buckets erhalten.',
  fieldMembersAdd: 'Speicher hinzufügen',
  fieldMembersEmpty:
    'Noch keine Mitglieder. Fügen Sie mindestens zwei Speicher hinzu.',
  fieldCopies: 'Gewünschte Kopien',
  fieldCopiesHelp: 'Zwischen 2 und der Anzahl der Mitglieder.',
  fieldMode: 'Failover-Modus',
  fieldModeManual: 'Manuell: nur Switchover',
  fieldModeAuto:
    'Automatisch: Failover, wenn die meisten Proxys den Primary als ausgefallen melden',
  fieldElectorate: 'Stimmberechtigte Proxys',
  fieldElectorateHelp:
    'Mindestens drei Proxys. Eine strikte Mehrheit muss den Primary als ausgefallen melden.',
  fieldAfter: 'Primary ausgefallen seit (Sekunden)',
  fieldAfterHelp: 'Standard 30 s.',
  fieldMaxLag: 'Max. Event-Lag',
  fieldMaxLagHelp:
    'Erlaubte nicht replizierte Events bei der Beförderung. Standard 0.',
  fieldSwitches: 'Max. Wechsel pro Stunde',
  fieldSwitchesHelp: 'Begrenzung für automatische Wechsel. Standard 3.',
  fieldAdopt: 'Vorhandene Replikationen übernehmen',
  fieldAdoptHelp:
    'Passende Replikationen vom Primary weiterverwenden, statt bei Vorhandensein zu scheitern.',
  immutableNote:
    'Bereich und Mitglieder sind unveränderlich. Gruppe lösen und neu erstellen, um sie zu ändern.',
  moveUp: 'Nach oben',
  moveDown: 'Nach unten',
  remove: 'Entfernen',
  save: 'Speichern',
  cancel: 'Abbrechen',
  saveFailed: 'Die Gruppe wurde nicht gespeichert: {message}',
  ID_REQUIRED: 'Geben Sie einen Gruppennamen ein.',
  ID_INVALID:
    'Der Name darf keine Leerzeichen, Doppelpunkte oder Schrägstriche enthalten.',
  USER_REQUIRED: 'Wählen Sie einen Benutzer.',
  MEMBERS_MIN: 'Wählen Sie mindestens zwei Mitgliedsspeicher.',
  COPIES_RANGE:
    'Kopien müssen zwischen 2 und der Anzahl der Mitglieder liegen.',
  BUCKET_INVALID:
    'Bucket-Namen müssen eindeutig und exakt sein, ohne Platzhalter.',
  LAG_NEGATIVE: 'Darf nicht negativ sein.',
  AFTER_NEGATIVE: 'Darf nicht negativ sein.',
  SWITCHES_POSITIVE: 'Muss mindestens 1 sein.',
  ELECTORATE_MIN:
    'Der automatische Modus braucht mindestens drei stimmberechtigte Proxys.',
  back: 'Schutzgruppen',
  notFound: 'Schutzgruppe nicht gefunden',
  notFoundText: 'Sie wurde möglicherweise gelöst. Zurück zur Liste.',
  edit: 'Bearbeiten',
  failoverNow: 'Jetzt Failover',
  switchover: 'Switchover',
  cancelSwitch: 'Switchover abbrechen',
  deleteGroup: 'Gruppe lösen',
  switchTitle: 'Failover für Gruppe {id}',
  switchPickTarget: 'Neuer Primary',
  switchPickPlaceholder: 'Mitglied auswählen',
  switchConsequence:
    'Schreibzugriffe pausieren kurz, bis {target} aufgeholt hat. Danach wird {target} zum Primary und {primary} zum Replikat.',
  switchConsequenceLag:
    '{target} hat noch nicht vollständig aufgeholt (Lag {lag}). Der Wechsel wartet darauf.',
  switchConfirm: 'Failover auf {target}',
  switchNoTargets:
    'Kein geeignetes Mitglied. Alle anderen Mitglieder sind ausgeschlossen.',
  switchFailed: 'Switchover abgelehnt: {message}',
  abortTitle: 'Umschaltung von {id} abbrechen?',
  abortText:
    'Der ausstehende Wechsel auf {target} wird verworfen und die Gruppe kehrt mit {primary} als Primary zu „bereit“ zurück. Ein bereits laufender Promote-Schritt kann nicht abgebrochen werden.',
  abortConfirm: 'Switchover abbrechen',
  abortFailed: 'Abbruch nicht möglich: {message}',
  deleteTitle: 'Gruppe {id} lösen?',
  deleteText:
    'Nur die Verwaltung wird entfernt. Routing, Replikationen und gespeicherte Objekte bleiben bestehen, aber die Gruppe hält keine Kopien mehr und führt kein Failover durch. Nur bereite Gruppen können gelöst werden.',
  deleteConfirm: 'Lösen',
  deleteFailed: 'Lösen nicht möglich: {message}',
  progressTitle: 'Switchover auf {target}',
  progressGeneric: 'Switchover läuft',
  'step.waiting': 'Warten auf Ziel',
  'step.draining': 'Schreibzugriffe leeren',
  'step.promoting': 'Beförderung',
  'step.resync': 'Erstkopie der neuen Replikate',
  colStorage: 'Speicher',
  colRole: 'Rolle',
  colHealth: 'Zustand',
  colMemberLag: 'Event-Lag',
  colInitialSync: 'Erstkopie',
  rolePrimary: 'Primary',
  roleTarget: 'Wechselziel',
  roleReplica: 'Replikat',
  'health.healthy': 'Aktuell',
  'health.syncing': 'Synchronisiert',
  'health.lagging': 'Im Rückstand',
  'health.unavailable': 'Nicht verfügbar',
  syncDone: 'Fertig',
  syncRunning: 'Läuft',
  writersEmpty:
    'Keine aktiven Writer. Eine eingefrorene Gruppe kann sofort befördern.',
  writersError: 'Writer konnten nicht geladen werden',
  writersHelp:
    'Eine eingefrorene Gruppe befördert erst, wenn kein Writer mehr übrig ist. Einträge abgestürzter Prozesse werden nach Ablauf ihres Leases automatisch entfernt.',
  colWriterOwner: 'Besitzer',
  colWriterKind: 'Art',
  colWriterSince: 'Zugelassen',
  colWriterInstance: 'Instanz',
  kindProxy: 'Proxy',
  kindWorker: 'Worker',
  clearWriter: 'Writer entfernen',
  clearTitle: 'Writer {owner} entfernen?',
  clearText:
    'Dies entfernt den Eintrag, der die Beförderung blockiert. Läuft der Prozess noch, kann sein Schreibzugriff nach dem Wechsel den alten Primary erreichen und Daten verlieren oder überschreiben. Fahren Sie nur fort, wenn der Prozess und seine Backend-Schreibzugriffe gestoppt sind.',
  clearAttest:
    'Ich bestätige, dass dieser Prozess und seine Schreibzugriffe auf das Backend gestoppt sind (extern isoliert).',
  clearOnlyFrozen: 'Nur verfügbar, solange die Gruppe eingefroren ist.',
  clearConfirm: 'Writer entfernen',
  clearFailed: 'Writer konnte nicht entfernt werden: {message}',
  historyEmpty: 'Noch keine Wechsel.',
  historyAutomatic: 'Automatisch',
  historyManual: 'Manuell',
  historyLine: '{from} → {to}',
  historyMeta: 'Verzug {lag}',
  lastErrorTitle: 'Letzter Controller-Fehler',
  lastErrorHint:
    'Der Controller versucht es jede Sekunde erneut. Die Meldung verschwindet, sobald ein Schritt gelingt.',
  settingCopies: 'Gewünschte Kopien',
  settingMode: 'Modus',
  settingAfter: 'Primary ausgefallen seit',
  settingLag: 'Max. Event-Lag',
  settingSwitches: 'Max. Wechsel pro Stunde',
  settingElectorate: 'Stimmberechtigte Proxys',
  settingSeconds: '{value} s',
  frozenBanner:
    'Schreibzugriffe auf diesen Bereich sind pausiert (eingefroren). Die Gruppe macht automatisch weiter, sobald laufende Schreibvorgänge beendet sind.',
  frozenBannerTitle: 'Gruppe ist eingefroren',
  quarantineBanner:
    'Ausgeschlossene Mitglieder werden nie automatisch überschrieben: {members}.',
  freezeSwitch: 'Automatisches Failover',
  freezeSwitchOn: 'Aktiv',
  freezeSwitchOff: 'Eingefroren',
  freezeErrorTitle: 'Automatisches Failover konnte nicht geändert werden',
  wizTitle: 'Schutzgruppe erstellen',
  wizStepScope: 'Umfang',
  wizStepMembers: 'Speicher',
  wizMembersDesc:
    'Der erste Speicher ist der Primary. Wählen Sie die Anzahl der Kopien.',
  wizFailoverTitle: 'Failover',
  wizOptionsTitle: 'Optionen',
  wizFinish: 'Gruppe erstellen',
  wizAllBuckets: 'Alle Buckets',
  wizSaveFailed: 'Die Gruppe wurde nicht erstellt',
  tabMembers: 'Mitglieder',
  tabWriters: 'Writer',
  tabActivity: 'Aktivität',
  tabSettings: 'Einstellungen',
  dangerTitle: 'Gefahrenzone',
  dangerDesc: 'Beim Lösen endet die Failover-Verwaltung der Gruppe.',
  editTitle: 'Einstellungen bearbeiten',
  editDesc: 'Umfang und Mitglieder sind nach dem Erstellen fest.',
  editAction: 'Einstellungen bearbeiten',
  colStorages: 'Speicher',
  search: 'Nach Name oder Speicher suchen',
  'filter.all': 'Alle',
  'filter.ready': 'Bereit',
  'filter.attention': 'Prüfen',
  noMatch: 'Keine Schutzgruppen gefunden',
  clearFilters: 'Filter zurücksetzen',
  overlapError:
    'Eine Replikation deckt diese Buckets bereits ab. Wählen Sie „Vorhandene Replikationen übernehmen“ oder andere Buckets.',
} as Record<string, string>;
