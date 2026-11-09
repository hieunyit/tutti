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
