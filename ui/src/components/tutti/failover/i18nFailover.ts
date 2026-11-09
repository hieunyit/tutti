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

export default <I18nMessages>{
  [I18nLocale.EN]: {
    policiesTitle: 'Failover policies',
    eventsTitle: 'Failover events',
    help: 'Automatic failover only happens when the proxies run with failover.enabled and only to the listed targets, in the order shown. Failback is manual: after you have repaired the source and synchronised it, acknowledge the event.',
    disabledWarning:
      'Automatic failover is disabled on {count} proxies; policies are stored but inactive.',
    errorTitle: 'Failed to load failover data',
    errorText: 'Policies or events could not be loaded.',
    retry: 'Retry',
    addPolicy: 'Add policy',
    editPolicy: 'Edit policy',
    columnSource: 'Source',
    columnEnabled: 'Enabled',
    columnTargets: 'Targets (in order)',
    columnAfter: 'After',
    columnMaxEventLag: 'Max event lag',
    columnActions: 'Actions',
    defaultValue: 'proxy default',
    edit: 'Edit',
    delete: 'Delete',
    deleteConfirm: 'Delete the failover policy for "{source}"?',
    deleteConfirmAction: 'Delete',
    cancel: 'Cancel',
    save: 'Save',
    noPolicies: 'No failover policies configured.',
    deleteFailed: 'Could not delete the policy.',
    saveFailed: 'Could not save the policy.',
    saveUnknownError: 'Unknown error while saving the policy.',
    fieldSource: 'Source storage',
    fieldEnabled: 'Enabled',
    fieldTargets: 'Targets (priority order)',
    fieldAfter: 'Fail over after',
    fieldAfterHint:
      'Seconds or a value like 60s, 5m, 1h. Empty = proxy default.',
    fieldMaxEventLag: 'Max event lag',
    fieldMaxEventLagHint:
      'Maximum number of unreplicated events allowed. Empty = proxy default.',
    addTarget: 'Add target',
    selectTarget: 'Select storage',
    selectSource: 'Select storage',
    noTargets: 'No targets selected.',
    moveUp: 'Move up',
    moveDown: 'Move down',
    remove: 'Remove',
    SOURCE_REQUIRED: 'Select a source storage.',
    TARGETS_REQUIRED: 'An enabled policy needs at least one target.',
    TARGET_IS_SOURCE: 'A target must differ from the source.',
    TARGET_DUPLICATE: 'Targets must be unique.',
    AFTER_INVALID: 'Enter a non-negative duration such as 60, 60s or 5m.',
    MAX_EVENT_LAG_INVALID: 'Max event lag must be a non-negative integer.',
    columnUser: 'User',
    columnBucket: 'Bucket',
    columnFromTo: 'From → To',
    columnTime: 'Time',
    columnEventLag: 'Event lag at failover',
    allBuckets: 'all buckets',
    acknowledge: 'Acknowledge',
    acknowledgeConfirm:
      'Acknowledge the failover of "{user}" ({bucket}) from {from} to {to}? Do this only after the manual failback is complete.',
    acknowledgeFailed: 'Could not acknowledge the event.',
    noEvents: 'No failover events.',
    eventsHint:
      'After repairing the source, perform the manual failback and then acknowledge the event. See docs/failover.md.',
  },
  [I18nLocale.DE]: {
    policiesTitle: 'Failover-Richtlinien',
    eventsTitle: 'Failover-Ereignisse',
    help: 'Automatisches Failover findet nur statt, wenn die Proxys mit failover.enabled laufen, und nur zu den aufgeführten Zielen in der angezeigten Reihenfolge. Failback erfolgt manuell: Bestätigen Sie das Ereignis, nachdem die Quelle repariert und synchronisiert wurde.',
    disabledWarning:
      'Automatisches Failover ist auf {count} Proxys deaktiviert; Richtlinien werden gespeichert, sind aber inaktiv.',
    errorTitle: 'Failover-Daten konnten nicht geladen werden',
    errorText: 'Richtlinien oder Ereignisse konnten nicht geladen werden.',
    retry: 'Erneut versuchen',
    addPolicy: 'Richtlinie hinzufügen',
    editPolicy: 'Richtlinie bearbeiten',
    columnSource: 'Quelle',
    columnEnabled: 'Aktiv',
    columnTargets: 'Ziele (in Reihenfolge)',
    columnAfter: 'Nach',
    columnMaxEventLag: 'Max. Event-Rückstand',
    columnActions: 'Aktionen',
    defaultValue: 'Proxy-Standard',
    edit: 'Bearbeiten',
    delete: 'Löschen',
    deleteConfirm: 'Failover-Richtlinie für "{source}" löschen?',
    deleteConfirmAction: 'Löschen',
    cancel: 'Abbrechen',
    save: 'Speichern',
    noPolicies: 'Keine Failover-Richtlinien konfiguriert.',
    deleteFailed: 'Die Richtlinie konnte nicht gelöscht werden.',
    saveFailed: 'Die Richtlinie konnte nicht gespeichert werden.',
    saveUnknownError: 'Unbekannter Fehler beim Speichern der Richtlinie.',
    fieldSource: 'Quellspeicher',
    fieldEnabled: 'Aktiv',
    fieldTargets: 'Ziele (Prioritätsreihenfolge)',
    fieldAfter: 'Failover nach',
    fieldAfterHint:
      'Sekunden oder Wert wie 60s, 5m, 1h. Leer = Proxy-Standard.',
    fieldMaxEventLag: 'Max. Event-Rückstand',
    fieldMaxEventLagHint:
      'Maximal erlaubte Anzahl nicht replizierter Events. Leer = Proxy-Standard.',
    addTarget: 'Ziel hinzufügen',
    selectTarget: 'Speicher wählen',
    selectSource: 'Speicher wählen',
    noTargets: 'Keine Ziele ausgewählt.',
    moveUp: 'Nach oben',
    moveDown: 'Nach unten',
    remove: 'Entfernen',
    SOURCE_REQUIRED: 'Wählen Sie einen Quellspeicher.',
    TARGETS_REQUIRED: 'Eine aktive Richtlinie benötigt mindestens ein Ziel.',
    TARGET_IS_SOURCE: 'Ein Ziel muss sich von der Quelle unterscheiden.',
    TARGET_DUPLICATE: 'Ziele müssen eindeutig sein.',
    AFTER_INVALID:
      'Geben Sie eine nicht negative Dauer wie 60, 60s oder 5m ein.',
    MAX_EVENT_LAG_INVALID:
      'Der Event-Rückstand muss eine nicht negative ganze Zahl sein.',
    columnUser: 'Benutzer',
    columnBucket: 'Bucket',
    columnFromTo: 'Von → Nach',
    columnTime: 'Zeit',
    columnEventLag: 'Event-Rückstand beim Failover',
    allBuckets: 'alle Buckets',
    acknowledge: 'Bestätigen',
    acknowledgeConfirm:
      'Failover von "{user}" ({bucket}) von {from} nach {to} bestätigen? Tun Sie dies erst nach abgeschlossenem manuellem Failback.',
    acknowledgeFailed: 'Das Ereignis konnte nicht bestätigt werden.',
    noEvents: 'Keine Failover-Ereignisse.',
    eventsHint:
      'Führen Sie nach der Reparatur der Quelle das manuelle Failback durch und bestätigen Sie dann das Ereignis. Siehe docs/failover.md.',
  },
};
