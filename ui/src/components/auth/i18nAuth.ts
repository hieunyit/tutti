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
    signInTo: 'Sign in to Tutti',
    loginDescription: 'Use the account your administrator created for you.',
    aboutTutti: 'About Tutti',
    brandHeadline: 'One S3 endpoint. Every copy accounted for.',
    brandLead:
      'Replicate, route and fail over S3 storage behind a single proxy, and see the health of every copy in one console.',
    brandPoint1: 'Synchronous or async replication across storages',
    brandPoint2: 'Health checks, circuit breakers and policy-based failover',
    brandPoint3: 'Per-account permissions and rotating access keys',
    brandBasedOn: 'based on Chorus by Clyso GmbH',
    errorInvalidCredentialsAttempts:
      'Wrong username or password. {n} attempts left before a lock.',
    show: 'Show',
    hide: 'Hide',
    showPassword: 'Show password',
    hidePassword: 'Hide password',
    signingIn: 'Signing in',
    loginHelp:
      'Forgot your password? Ask an administrator to reset it in the worker config.',
    usernamePlaceholder: 'Enter your username',
    passwordPlaceholder: 'Enter your password',
    username: 'Username',
    password: 'Password',
    keepSignedIn: 'Keep me signed in',
    keepSignedInHint:
      'Only on a device you trust. Keeps you signed in on this device until the session expires.',
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
    changePassword: 'Change password',
    userMenu: 'User menu',
    signedInAs: 'Signed in as',
    roleInfoAdmin: 'Administrator: full read and write access.',
    roleInfoViewer: 'Viewer: read-only access.',
    roleAdmin: 'admin',
    roleViewer: 'viewer',
    roleNameAdmin: 'Administrator',
    roleNameViewer: 'Viewer',
    readOnlyAccount: 'read-only account',
  },
  [I18nLocale.DE]: {
    loginTitle: 'Anmelden',
    loginSubtitle: 'Melden Sie sich an, um Tutti zu verwalten.',
    signInTo: 'Bei Tutti anmelden',
    loginDescription:
      'Verwenden Sie das Konto, das Ihr Administrator für Sie angelegt hat.',
    aboutTutti: 'Über Tutti',
    brandHeadline: 'Ein S3-Endpunkt. Jede Kopie im Blick.',
    brandLead:
      'Replizieren, routen und Failover für S3-Speicher hinter einem einzigen Proxy, und der Zustand jeder Kopie in einer Konsole.',
    brandPoint1: 'Synchrone oder asynchrone Replikation zwischen Speichern',
    brandPoint2:
      'Health-Checks, Circuit Breaker und richtlinienbasiertes Failover',
    brandPoint3: 'Berechtigungen pro Konto und rotierende Zugriffsschlüssel',
    brandBasedOn: 'basiert auf Chorus von Clyso GmbH',
    errorInvalidCredentialsAttempts:
      'Benutzername oder Passwort ist falsch. Noch {n} Versuche bis zur Sperre.',
    show: 'Anzeigen',
    hide: 'Verbergen',
    showPassword: 'Passwort anzeigen',
    hidePassword: 'Passwort verbergen',
    signingIn: 'Anmeldung läuft',
    loginHelp:
      'Passwort vergessen? Bitten Sie einen Administrator, es in der Worker-Konfiguration zurückzusetzen.',
    usernamePlaceholder: 'Benutzernamen eingeben',
    passwordPlaceholder: 'Passwort eingeben',
    username: 'Benutzername',
    password: 'Passwort',
    keepSignedIn: 'Angemeldet bleiben',
    keepSignedInHint:
      'Nur auf einem vertrauenswürdigen Gerät. Hält Sie auf diesem Gerät angemeldet, bis die Sitzung abläuft.',
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
    changePassword: 'Passwort ändern',
    userMenu: 'Benutzermenü',
    signedInAs: 'Angemeldet als',
    roleInfoAdmin: 'Administrator: voller Lese- und Schreibzugriff.',
    roleInfoViewer: 'Betrachter: nur Lesezugriff.',
    roleAdmin: 'Admin',
    roleViewer: 'Betrachter',
    roleNameAdmin: 'Administrator',
    roleNameViewer: 'Betrachter',
    readOnlyAccount: 'Konto nur mit Lesezugriff',
  },
};
