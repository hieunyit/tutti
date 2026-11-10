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
    title: 'Documentation',
    description:
      'Guides shipped with this release, and the management API reference generated from its OpenAPI spec.',
    sections: 'Documentation sections',
    tabGuides: 'Guides',
    tabApi: 'API reference',
    guidesNav: 'Guides',
    search: 'Search docs',
    searchPlaceholder: 'Search docs',
    searchEmpty: 'No guide matches "{query}".',
    searchLoading: 'Searching…',
    breadcrumb: 'Guides',
    viewSource: 'View on GitHub',
    loading: 'Loading guide…',
    notFoundTitle: 'Guide not found',
    notFoundText: 'There is no guide called "{slug}".',
    notBundledTitle: 'Guides are not part of this build',
    notBundledText:
      'This UI was built without the repository docs. Read them on GitHub instead.',
    loadFailed: 'The guide could not be loaded.',
    retry: 'Retry',
    openFirst: 'Open the first guide',
    openOnGithub: 'Open on GitHub',
    guide_operations: 'Operations',
    guide_configuration: 'Configuration',
    'guide_storage-accounts': 'Storage accounts',
    'guide_resilience-api': 'Resilience API and CLI',
    'guide_upgrading-from-chorus': 'Upgrading from Chorus',
    'guide_enterprise-ui': 'Enterprise UI',
    guide_failover: 'Failover',
    apiTitle: 'API reference',
    apiDescription:
      'Endpoints of the Tutti management API, generated from the bundled OpenAPI spec.',
    apiSpecMeta: 'OpenAPI 2.0 · {prefix}',
    downloadSpec: 'Download spec',
    apiHint:
      'Try requests in place with your current session. Admin-only calls are marked.',
    apiLoading: 'Loading API reference…',
    apiFailed: 'The API reference could not be loaded.',
    apiNotBundledTitle: 'The API spec is not part of this build',
    apiNotBundledText:
      'This UI was built without the OpenAPI spec (proto/gen/openapi/chorus/chorus.swagger.json).',
    roleAdmin: 'admin only',
    roleViewer: 'viewer',
    rolePublic: 'public',
    roleLegend: 'Who may call an operation',
  },
  [I18nLocale.DE]: {
    title: 'Dokumentation',
    description:
      'Anleitungen dieser Version und die Referenz der Management-API, erzeugt aus der OpenAPI-Spezifikation.',
    sections: 'Dokumentationsbereiche',
    tabGuides: 'Anleitungen',
    tabApi: 'API-Referenz',
    guidesNav: 'Anleitungen',
    search: 'Dokumentation durchsuchen',
    searchPlaceholder: 'Dokumentation durchsuchen',
    searchEmpty: 'Keine Anleitung passt zu "{query}".',
    searchLoading: 'Suche läuft…',
    breadcrumb: 'Anleitungen',
    viewSource: 'Auf GitHub ansehen',
    loading: 'Anleitung wird geladen…',
    notFoundTitle: 'Anleitung nicht gefunden',
    notFoundText: 'Es gibt keine Anleitung "{slug}".',
    notBundledTitle: 'Anleitungen sind nicht Teil dieses Builds',
    notBundledText:
      'Diese Oberfläche wurde ohne die Repository-Dokumentation gebaut. Lesen Sie sie stattdessen auf GitHub.',
    loadFailed: 'Die Anleitung konnte nicht geladen werden.',
    retry: 'Erneut versuchen',
    openFirst: 'Erste Anleitung öffnen',
    openOnGithub: 'Auf GitHub öffnen',
    guide_operations: 'Betrieb',
    guide_configuration: 'Konfiguration',
    'guide_storage-accounts': 'Speicherkonten',
    'guide_resilience-api': 'Resilience-API und CLI',
    'guide_upgrading-from-chorus': 'Umstieg von Chorus',
    'guide_enterprise-ui': 'Enterprise-Oberfläche',
    guide_failover: 'Failover',
    apiTitle: 'API-Referenz',
    apiDescription:
      'Endpunkte der Tutti-Management-API, erzeugt aus der mitgelieferten OpenAPI-Spezifikation.',
    apiSpecMeta: 'OpenAPI 2.0 · {prefix}',
    downloadSpec: 'Spezifikation herunterladen',
    apiHint:
      'Anfragen direkt mit Ihrer aktuellen Sitzung ausprobieren. Nur-Admin-Aufrufe sind markiert.',
    apiLoading: 'API-Referenz wird geladen…',
    apiFailed: 'Die API-Referenz konnte nicht geladen werden.',
    apiNotBundledTitle: 'Die API-Spezifikation ist nicht Teil dieses Builds',
    apiNotBundledText:
      'Diese Oberfläche wurde ohne die OpenAPI-Spezifikation gebaut (proto/gen/openapi/chorus/chorus.swagger.json).',
    roleAdmin: 'nur Admin',
    roleViewer: 'Viewer',
    rolePublic: 'öffentlich',
    roleLegend: 'Wer eine Operation aufrufen darf',
  },
};
