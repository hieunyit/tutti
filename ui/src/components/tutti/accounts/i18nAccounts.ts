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
    title: 'Proxy accounts',
    banner:
      'Access policies apply on every proxy within about 5 seconds. Accounts without a policy keep full access (read-write on all buckets).',
    errorTitle: 'Failed to load proxy accounts',
    errorText: 'The account list could not be loaded.',
    retry: 'Retry',
    noAccounts: 'No proxy accounts found.',
    columnUser: 'User',
    columnKeys: 'Access keys',
    columnPermission: 'Permission',
    columnBuckets: 'Allowed buckets',
    columnActions: 'Actions',
    allBuckets: 'all buckets',
    noPolicyHint: 'No policy: full access',
    sourceCustom: 'custom',
    permissionReadWrite: 'read-write',
    permissionReadOnly: 'read-only',
    permissionWriteOnly: 'write-only',
    permissionReadWriteHint:
      'Read-write: the account can read, list, upload, delete and manage objects.',
    permissionReadOnlyHint:
      'Read-only: the account can read and list objects but every write, delete or management call is denied.',
    permissionWriteOnlyHint:
      'Write-only: the account can upload and delete objects but cannot read or list them (useful for ingest or backup agents).',
    editPolicy: 'Edit policy',
    createPolicy: 'Set policy',
    removePolicy: 'Remove policy',
    removeConfirm:
      'Remove the policy of "{user}"? The account gets full access again.',
    removeConfirmAction: 'Remove',
    removeFailed: 'Could not remove the policy.',
    cancel: 'Cancel',
    save: 'Save',
    fieldUser: 'User',
    fieldPermission: 'Permission',
    fieldBuckets: 'Allowed buckets',
    fieldBucketsHint:
      'Glob patterns such as photos-* or logs. * or an empty list means all buckets.',
    bucketPlaceholder: 'bucket or pattern, e.g. photos-*',
    addPattern: 'Add',
    removePattern: 'Remove pattern',
    noPatterns: 'No patterns: all buckets are allowed.',
    fieldDescription: 'Description',
    saveUnknownError: 'Unknown error while saving the policy.',
    USER_REQUIRED: 'A user is required.',
    PERMISSION_INVALID: 'Choose a permission.',
    PATTERN_EMPTY: 'A bucket pattern must not be empty.',
    PATTERN_WHITESPACE: 'A bucket pattern must not contain spaces.',
    PATTERN_DUPLICATE: 'Bucket patterns must be unique.',
  },
  [I18nLocale.DE]: {
    title: 'Proxy-Konten',
    banner:
      'Zugriffsrichtlinien gelten innerhalb von etwa 5 Sekunden auf jedem Proxy. Konten ohne Richtlinie behalten vollen Zugriff (Lesen und Schreiben auf allen Buckets).',
    errorTitle: 'Proxy-Konten konnten nicht geladen werden',
    errorText: 'Die Kontenliste konnte nicht geladen werden.',
    retry: 'Erneut versuchen',
    noAccounts: 'Keine Proxy-Konten gefunden.',
    columnUser: 'Benutzer',
    columnKeys: 'Zugriffsschlüssel',
    columnPermission: 'Berechtigung',
    columnBuckets: 'Erlaubte Buckets',
    columnActions: 'Aktionen',
    allBuckets: 'alle Buckets',
    noPolicyHint: 'Keine Richtlinie: voller Zugriff',
    sourceCustom: 'benutzerdefiniert',
    permissionReadWrite: 'Lesen und Schreiben',
    permissionReadOnly: 'nur Lesen',
    permissionWriteOnly: 'nur Schreiben',
    permissionReadWriteHint:
      'Lesen und Schreiben: Das Konto kann Objekte lesen, auflisten, hochladen, löschen und verwalten.',
    permissionReadOnlyHint:
      'Nur Lesen: Das Konto kann Objekte lesen und auflisten; jeder Schreib-, Lösch- oder Verwaltungsaufruf wird abgelehnt.',
    permissionWriteOnlyHint:
      'Nur Schreiben: Das Konto kann Objekte hochladen und löschen, aber nicht lesen oder auflisten (z. B. für Ingest- oder Backup-Agenten).',
    editPolicy: 'Richtlinie bearbeiten',
    createPolicy: 'Richtlinie setzen',
    removePolicy: 'Richtlinie entfernen',
    removeConfirm:
      'Richtlinie von "{user}" entfernen? Das Konto erhält wieder vollen Zugriff.',
    removeConfirmAction: 'Entfernen',
    removeFailed: 'Die Richtlinie konnte nicht entfernt werden.',
    cancel: 'Abbrechen',
    save: 'Speichern',
    fieldUser: 'Benutzer',
    fieldPermission: 'Berechtigung',
    fieldBuckets: 'Erlaubte Buckets',
    fieldBucketsHint:
      'Glob-Muster wie photos-* oder logs. * oder eine leere Liste bedeutet alle Buckets.',
    bucketPlaceholder: 'Bucket oder Muster, z. B. photos-*',
    addPattern: 'Hinzufügen',
    removePattern: 'Muster entfernen',
    noPatterns: 'Keine Muster: Alle Buckets sind erlaubt.',
    fieldDescription: 'Beschreibung',
    saveUnknownError: 'Unbekannter Fehler beim Speichern der Richtlinie.',
    USER_REQUIRED: 'Ein Benutzer ist erforderlich.',
    PERMISSION_INVALID: 'Wählen Sie eine Berechtigung.',
    PATTERN_EMPTY: 'Ein Bucket-Muster darf nicht leer sein.',
    PATTERN_WHITESPACE: 'Ein Bucket-Muster darf keine Leerzeichen enthalten.',
    PATTERN_DUPLICATE: 'Bucket-Muster müssen eindeutig sein.',
  },
};
