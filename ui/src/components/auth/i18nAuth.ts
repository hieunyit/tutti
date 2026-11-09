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
    loginTitle: 'Sign in',
    loginSubtitle: 'Sign in to manage your Tutti deployment.',
    username: 'Username',
    password: 'Password',
    keepSignedIn: 'Keep me signed in',
    keepSignedInHint:
      'Stores the session token on this device until it expires. Use only on trusted computers.',
    signIn: 'Sign in',
    usernameRequired: 'Enter your username.',
    passwordRequired: 'Enter your password.',
    errorInvalidCredentials: 'Wrong username or password.',
    errorRateLimited: 'Too many attempts. Try again in {time}.',
    errorRateLimitedUnknown: 'Too many attempts. Try again later.',
    errorUnknown: 'Sign in failed. Check your connection and try again.',
    secondsShort: '{n} s',
    minutesShort: '{n} min',
    logout: 'Sign out',
    userMenu: 'User menu',
    signedInAs: 'Signed in as',
    roleInfoAdmin: 'Administrator: full read and write access.',
    roleInfoViewer: 'Viewer: read-only access.',
    roleAdmin: 'admin',
    roleViewer: 'viewer',
    readOnlyAccount: 'read-only account',
  },
  [I18nLocale.DE]: {
    loginTitle: 'Anmelden',
    loginSubtitle: 'Melden Sie sich an, um Tutti zu verwalten.',
    username: 'Benutzername',
    password: 'Passwort',
    keepSignedIn: 'Angemeldet bleiben',
    keepSignedInHint:
      'Speichert das Sitzungstoken auf diesem Gerät bis zum Ablauf. Nur auf vertrauenswürdigen Computern verwenden.',
    signIn: 'Anmelden',
    usernameRequired: 'Geben Sie Ihren Benutzernamen ein.',
    passwordRequired: 'Geben Sie Ihr Passwort ein.',
    errorInvalidCredentials: 'Benutzername oder Passwort ist falsch.',
    errorRateLimited: 'Zu viele Versuche. Versuchen Sie es in {time} erneut.',
    errorRateLimitedUnknown:
      'Zu viele Versuche. Versuchen Sie es später erneut.',
    errorUnknown:
      'Anmeldung fehlgeschlagen. Prüfen Sie die Verbindung und versuchen Sie es erneut.',
    secondsShort: '{n} s',
    minutesShort: '{n} Min.',
    logout: 'Abmelden',
    userMenu: 'Benutzermenü',
    signedInAs: 'Angemeldet als',
    roleInfoAdmin: 'Administrator: voller Lese- und Schreibzugriff.',
    roleInfoViewer: 'Betrachter: nur Lesezugriff.',
    roleAdmin: 'Admin',
    roleViewer: 'Betrachter',
    readOnlyAccount: 'Konto nur mit Lesezugriff',
  },
};
