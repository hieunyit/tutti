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
    title: 'Dashboard',
    description:
      'Operations overview: storage health, replication progress and live traffic at a glance.',
    refresh: 'Refresh',
    updated: 'Updated {time}',
    notUpdated: 'Not updated yet',
    errorTitle: 'Failed to load the dashboard',
    errorText: 'The system status could not be loaded.',
    retry: 'Retry',
    partialError:
      'Some data could not be loaded and may be outdated: {sources}.',
    sourceStorages: 'storages',
    sourceReplications: 'replications',
    sourceProxies: 'proxy status',
    sourceEvents: 'failover events',

    attentionTitle: 'Needs attention',
    attentionCount: '{n} item | {n} items',
    allGood: 'All systems operational',
    severityError: 'Critical',
    severityWarning: 'Warning',
    showMore: 'Show {n} more',
    showLess: 'Show less',
    open: 'Open',
    storageDown:
      'Storage "{name}" is unavailable ({failures} consecutive failures).',
    failoverEvent:
      'Failover of "{user}" from {from} to {to} has not been acknowledged.',
    replicationPaused: 'Replication {name} is paused.',
    replicationBehind: 'Replication {name} is {lag} events behind.',
    proxyStale: 'Proxy "{name}" has no recent status update.',

    kpiStorages: 'Storages healthy',
    kpiStoragesHint: '{n} without health data',
    kpiStoragesAllKnown: 'Reported by the proxies',
    kpiReplications: 'Replications',
    kpiReplicationsActive: 'active',
    kpiReplicationsPaused: 'paused',
    kpiReplicationsHint: '{active} active, {paused} paused',
    kpiLag: 'Replication lag',
    kpiLagHint: 'Events behind, all replications',
    kpiLagNone: 'Everything in sync',
    kpiRequests: 'Requests/s',
    kpiRequestsTotalHint: '{n} requests since start',
    kpiRequestsWarmingUp: 'Rate available after the next refresh',
    kpiRequestsTotalLabel: 'Requests (total)',
    kpiErrors: 'Error rate (5xx)',
    kpiErrorsCurrent: 'Share of responses, current',
    kpiErrorsSinceStart: 'Share of responses, since start',
    kpiProxies: 'Proxies reporting',
    kpiProxiesHint: 'Reported within the last minute',
    kpiNoData: '-',

    storagesTitle: 'Storages',
    storagesManage: 'Manage',
    storagesEmpty: 'No storages configured.',
    storagesUnavailable: 'Storage data is currently unavailable.',
    storageMain: 'main',
    stateHealthy: 'Healthy',
    stateUnavailable: 'Unavailable',
    stateUnknown: 'Unknown',

    replicationsTitle: 'Replications',
    replicationsAll: 'All replications',
    replicationsAdd: 'Add replication',
    replicationsEmptyTitle: 'No replications configured',
    replicationsEmptyText:
      'Set up replication to automatically sync objects between storages and keep your data up to date.',
    breakdownLabel: 'Replication status distribution',
    breakdownTotal: '{n} in total',
    breakdownUpToDate: 'Up to date',
    breakdownInitial: 'Initial sync',
    breakdownBehind: 'Behind',
    breakdownPaused: 'Paused',
    columnStatus: 'Status',
    columnCount: 'Count',
    columnShare: 'Share',

    endpointTitle: 'S3 endpoint',
    endpointAddress: 'Address',
    endpointCredentials: 'Credentials',
    endpointSeeCredentials: 'See credentials',
    endpointAdminOnly: 'Only administrators can view proxy credentials.',
    endpointError: 'An error occurred while getting the proxy data.',
    endpointUnavailable: 'No proxy address available.',
    copy: 'Copy',
    copied: 'Copied',
    uptimeTitle: 'Uptime',
  },
  [I18nLocale.DE]: {
    title: 'Dashboard',
    description:
      'Betriebsübersicht: Speicherzustand, Replikationsfortschritt und Live-Traffic auf einen Blick.',
    refresh: 'Aktualisieren',
    updated: 'Aktualisiert {time}',
    notUpdated: 'Noch nicht aktualisiert',
    errorTitle: 'Dashboard konnte nicht geladen werden',
    errorText: 'Der Systemstatus konnte nicht geladen werden.',
    retry: 'Erneut versuchen',
    partialError:
      'Einige Daten konnten nicht geladen werden und sind evtl. veraltet: {sources}.',
    sourceStorages: 'Speicher',
    sourceReplications: 'Replikationen',
    sourceProxies: 'Proxy-Status',
    sourceEvents: 'Failover-Ereignisse',

    attentionTitle: 'Handlungsbedarf',
    attentionCount: '{n} Eintrag | {n} Einträge',
    allGood: 'Alle Systeme laufen normal',
    severityError: 'Kritisch',
    severityWarning: 'Warnung',
    showMore: '{n} weitere anzeigen',
    showLess: 'Weniger anzeigen',
    open: 'Öffnen',
    storageDown:
      'Speicher "{name}" ist nicht verfügbar ({failures} Fehler in Folge).',
    failoverEvent:
      'Failover von "{user}" von {from} nach {to} wurde nicht quittiert.',
    replicationPaused: 'Replikation {name} ist pausiert.',
    replicationBehind: 'Replikation {name} liegt {lag} Ereignisse zurück.',
    proxyStale: 'Für Proxy "{name}" liegt keine aktuelle Statusmeldung vor.',

    kpiStorages: 'Speicher intakt',
    kpiStoragesHint: '{n} ohne Zustandsdaten',
    kpiStoragesAllKnown: 'Von den Proxys gemeldet',
    kpiReplications: 'Replikationen',
    kpiReplicationsActive: 'aktiv',
    kpiReplicationsPaused: 'pausiert',
    kpiReplicationsHint: '{active} aktiv, {paused} pausiert',
    kpiLag: 'Replikations-Rückstand',
    kpiLagHint: 'Offene Ereignisse, alle Replikationen',
    kpiLagNone: 'Alles synchron',
    kpiRequests: 'Anfragen/s',
    kpiRequestsTotalHint: '{n} Anfragen seit Start',
    kpiRequestsWarmingUp: 'Rate nach der nächsten Aktualisierung verfügbar',
    kpiRequestsTotalLabel: 'Anfragen (gesamt)',
    kpiErrors: 'Fehlerrate (5xx)',
    kpiErrorsCurrent: 'Anteil der Antworten, aktuell',
    kpiErrorsSinceStart: 'Anteil der Antworten, seit Start',
    kpiProxies: 'Proxys aktiv',
    kpiProxiesHint: 'Innerhalb der letzten Minute gemeldet',
    kpiNoData: '-',

    storagesTitle: 'Speicher',
    storagesManage: 'Verwalten',
    storagesEmpty: 'Keine Speicher konfiguriert.',
    storagesUnavailable: 'Speicherdaten sind derzeit nicht verfügbar.',
    storageMain: 'Haupt',
    stateHealthy: 'Intakt',
    stateUnavailable: 'Nicht verfügbar',
    stateUnknown: 'Unbekannt',

    replicationsTitle: 'Replikationen',
    replicationsAll: 'Alle Replikationen',
    replicationsAdd: 'Replikation hinzufügen',
    replicationsEmptyTitle: 'Keine Replikationen konfiguriert',
    replicationsEmptyText:
      'Richten Sie eine Replikation ein, um Objekte automatisch zwischen Speichern zu synchronisieren und Ihre Daten aktuell zu halten.',
    breakdownLabel: 'Verteilung der Replikationsstatus',
    breakdownTotal: '{n} insgesamt',
    breakdownUpToDate: 'Aktuell',
    breakdownInitial: 'Erstsynchronisation',
    breakdownBehind: 'Im Rückstand',
    breakdownPaused: 'Pausiert',
    columnStatus: 'Status',
    columnCount: 'Anzahl',
    columnShare: 'Anteil',

    endpointTitle: 'S3-Endpunkt',
    endpointAddress: 'Adresse',
    endpointCredentials: 'Zugangsdaten',
    endpointSeeCredentials: 'Zugangsdaten anzeigen',
    endpointAdminOnly:
      'Nur Administratoren können die Proxy-Anmeldeinformationen sehen.',
    endpointError: 'Beim Abrufen der Proxy-Daten ist ein Fehler aufgetreten.',
    endpointUnavailable: 'Keine Proxy-Adresse verfügbar.',
    copy: 'Kopieren',
    copied: 'Kopiert',
    uptimeTitle: 'Verfügbarkeit',
  },
};
