/*
 * Copyright © 2026 Clyso GmbH
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
    pageDescription:
      'The storage backends behind the proxy. Open a storage to see its details and manage the credentials of its users.',
    refresh: 'Refresh',
    noFollowersTitle: 'No follower storages',
    noFollowersText:
      'Only the main storage is configured. Add follower storages in the configuration to replicate to them.',
    storagesTitle: 'Storages',
    mainStorageTitle: 'Main',
    followersStoragesTitle: 'Followers',
    storagesErrorTitle: 'Storages Error',
    storagesErrorText:
      'An error occurred while retrieving storages.\nPlease try again.',
    storagesErrorAction: 'Retry',

    storageGeneralTitle: 'General Information',
    mainStorage: 'Main',
    followerStorage: 'Follower',
    nameLabel: 'Name',
    addressLabel: 'Address',
    providerLabel: 'Provider',
    typeLabel: 'Type',

    storageBrowserTitle: 'Storage Explorer',
  },
  [I18nLocale.DE]: {
    pageDescription:
      'Die Speicher-Backends hinter dem Proxy. Öffnen Sie einen Speicher, um Details einzusehen und die Zugangsdaten seiner Benutzer zu verwalten.',
    refresh: 'Aktualisieren',
    noFollowersTitle: 'Keine Follower-Speicher',
    noFollowersText:
      'Es ist nur der Hauptspeicher konfiguriert. Fügen Sie Follower-Speicher in der Konfiguration hinzu, um dorthin zu replizieren.',
    storagesTitle: 'Speicher',
    mainStorageTitle: 'Hauptspeicher',
    followersStoragesTitle: 'Follower',
    storagesErrorTitle: 'Fehler bei Storages',
    storagesErrorText:
      'Beim Abrufen der Speicher ist ein Fehler aufgetreten.\nBitte versuchen Sie es erneut.',
    storagesErrorAction: 'Erneut versuchen',

    storageGeneralTitle: 'Allgemeine Informationen',
    mainStorage: 'Hauptspeicher',
    followerStorage: 'Follower',
    nameLabel: 'Name',
    addressLabel: 'Adresse',
    providerLabel: 'Anbieter',
    typeLabel: 'Typ',

    storageBrowserTitle: 'Storage Explorer',
  },
};
