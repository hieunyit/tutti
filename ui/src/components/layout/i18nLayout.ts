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
    navLabel: 'Main navigation',
    'group.overview': 'Overview',
    'group.storage': 'Storage',
    'group.dataProtection': 'Data protection',
    'group.access': 'Access',
    'group.administration': 'Administration',
    'group.help': 'Help',
    consoleTagline: 'S3 replication console',
    aboutTutti: 'About Tutti',
    badgeLabel: 'open items',
    searchLabel: 'Jump to page',
    searchPlaceholder: 'Jump to page',
    searchResults: 'Pages',
    searchNoResults: 'No matching page',
    themeDark: 'Switch to dark theme',
    themeLight: 'Switch to light theme',
    authDisabledBanner:
      'Sign-in is disabled: anyone who can reach this console has admin access. Enable api.auth in the worker config.',
    authDisabledLink: 'Read the documentation',
    openMenu: 'Open menu',
    closeMenu: 'Close menu',
    collapse: 'Collapse sidebar',
    expand: 'Expand sidebar',
    breadcrumbs: 'Breadcrumbs',
    about: 'About',
    aboutTitle: 'About Tutti',
    version: 'Version',
    productTagline: 'S3 proxy and replication control plane',
    attributionBefore: 'Tutti is based on',
    attributionAfter:
      'by Clyso GmbH (https://github.com/clyso/chorus), licensed under AGPL-3.0 (UI) and Apache-2.0 (backend).',
    sourceCode: 'Source code on GitHub',
    close: 'Close',
  },
  [I18nLocale.DE]: {
    navLabel: 'Hauptnavigation',
    'group.overview': 'Übersicht',
    'group.storage': 'Speicher',
    'group.dataProtection': 'Datenschutz',
    'group.access': 'Zugriff',
    'group.administration': 'Verwaltung',
    'group.help': 'Hilfe',
    consoleTagline: 'S3-Replikationskonsole',
    aboutTutti: 'Über Tutti',
    badgeLabel: 'offene Einträge',
    searchLabel: 'Zu Seite springen',
    searchPlaceholder: 'Zu Seite springen',
    searchResults: 'Seiten',
    searchNoResults: 'Keine passende Seite',
    themeDark: 'Zum dunklen Design wechseln',
    themeLight: 'Zum hellen Design wechseln',
    authDisabledBanner:
      'Die Anmeldung ist deaktiviert: Jeder, der diese Konsole erreichen kann, hat Administratorzugriff. Aktivieren Sie api.auth in der Worker-Konfiguration.',
    authDisabledLink: 'Dokumentation lesen',
    openMenu: 'Menü öffnen',
    closeMenu: 'Menü schließen',
    collapse: 'Seitenleiste einklappen',
    expand: 'Seitenleiste ausklappen',
    breadcrumbs: 'Brotkrumennavigation',
    about: 'Info',
    aboutTitle: 'Über Tutti',
    version: 'Version',
    productTagline: 'S3-Proxy und Replikations-Steuerung',
    attributionBefore: 'Tutti basiert auf',
    attributionAfter:
      'von Clyso GmbH (https://github.com/clyso/chorus), lizenziert unter AGPL-3.0 (UI) und Apache-2.0 (Backend).',
    sourceCode: 'Quellcode auf GitHub',
    close: 'Schließen',
  },
};
