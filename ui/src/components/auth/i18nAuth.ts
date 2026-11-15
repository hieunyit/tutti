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

// Plain locale codes keep this file free of the @clyso/clyso-ui-kit bundle.
export default <Record<string, Record<string, string>>>{
  en: {
    currentPassword: 'Current password',
    newPassword: 'New password',
    repeatPassword: 'Repeat new password',
    mismatch: 'Passwords do not match.',
    passwordInvalid:
      'Use at least 12 characters (at most 72 bytes), different from the username.',
    passwordHint: 'Other sessions will be signed out.',
    passwordRules: 'Password rules',
    ruleLength: 'At least 12 characters, at most 72 bytes',
    ruleMatch: 'Both new password fields match',
    forcedHint: 'Set a new password before continuing to the console.',
    passwordStrength: 'Strength: {strength}',
    weak: 'Weak',
    fair: 'Fair',
    strong: 'Strong',
    passwordChanged: 'Password updated.',
    changePasswordFailed: 'Could not change the password. Try again.',
    changingPassword: 'Changing password',
    loginTitle: 'Sign in',
    errorInvalidCredentialsAttempts:
      'The username and password do not match. {n} attempts left before a temporary lock.',
    signingIn: 'Signing in',
    loginHelp: 'Forgot your password or signing in for the first time?',
    firstRunHint: 'See how to get the admin password.',
    username: 'Username',
    password: 'Password',
    keepSignedIn: 'Keep me signed in',
    signIn: 'Sign in',
    usernameRequired: 'Enter your username.',
    passwordRequired: 'Enter your password.',
    errorInvalidCredentials: 'The username and password do not match.',
    errorRateLimited: 'Too many sign-in attempts. Try again in {time}.',
    errorRateLimitedUnknown: 'Too many sign-in attempts. Try again later.',
    errorUnknown:
      'We could not sign you in. Check the connection and try again.',
    connectionUnavailable:
      'The server could not be reached, so access could not be checked. Retry to continue.',
    retryConnection: 'Retry connection',
    secondsShort: '{n} s',
    minutesShort: '{n} min',
    logout: 'Sign out',
    changePassword: 'Change password',
    close: 'Close',
    configAccountText:
      'The account {name} is defined in the server configuration, so its password cannot be changed here. Update the password hash in the worker config file and restart the worker.',
    userMenu: 'User menu',
    signedInAs: 'Signed in as',
    roleInfoAdmin: 'Administrator: full read and write access.',
    roleInfoViewer: 'Viewer: read-only access.',
    roleNameAdmin: 'Administrator',
    roleNameViewer: 'Viewer',
    readOnlyAccount: 'read-only account',
  },
  de: {
    currentPassword: 'Aktuelles Passwort',
    newPassword: 'Neues Passwort',
    repeatPassword: 'Neues Passwort wiederholen',
    mismatch: 'Passwörter stimmen nicht überein.',
    passwordInvalid:
      'Verwenden Sie mindestens 12 Zeichen (höchstens 72 Bytes), anders als der Benutzername.',
    passwordHint: 'Andere Sitzungen werden abgemeldet.',
    passwordRules: 'Passwortregeln',
    ruleLength: 'Mindestens 12 Zeichen, höchstens 72 Bytes',
    ruleMatch: 'Beide Felder für das neue Passwort stimmen überein',
    forcedHint: 'Setzen Sie ein neues Passwort, bevor Sie fortfahren.',
    passwordStrength: 'Stärke: {strength}',
    weak: 'Schwach',
    fair: 'Mittel',
    strong: 'Stark',
    passwordChanged: 'Passwort aktualisiert.',
    changePasswordFailed:
      'Das Passwort konnte nicht geändert werden. Versuchen Sie es erneut.',
    changingPassword: 'Passwort wird geändert',
    loginTitle: 'Anmelden',
    errorInvalidCredentialsAttempts:
      'Benutzername und Passwort passen nicht zusammen. Noch {n} Versuche bis zur vorübergehenden Sperre.',
    signingIn: 'Anmeldung läuft',
    loginHelp: 'Passwort vergessen oder erste Anmeldung?',
    firstRunHint: 'So erhalten Sie das Admin-Passwort.',
    username: 'Benutzername',
    password: 'Passwort',
    keepSignedIn: 'Angemeldet bleiben',
    signIn: 'Anmelden',
    usernameRequired: 'Geben Sie Ihren Benutzernamen ein.',
    passwordRequired: 'Geben Sie Ihr Passwort ein.',
    errorInvalidCredentials: 'Benutzername und Passwort passen nicht zusammen.',
    errorRateLimited:
      'Zu viele Anmeldeversuche. Versuchen Sie es in {time} erneut.',
    errorRateLimitedUnknown:
      'Zu viele Anmeldeversuche. Versuchen Sie es später erneut.',
    errorUnknown:
      'Die Anmeldung war nicht möglich. Prüfen Sie die Verbindung und versuchen Sie es erneut.',
    connectionUnavailable:
      'Der Server ist nicht erreichbar, daher konnte der Zugriff nicht geprüft werden. Erneut versuchen, um fortzufahren.',
    retryConnection: 'Verbindung erneut prüfen',
    secondsShort: '{n} s',
    minutesShort: '{n} Min.',
    logout: 'Abmelden',
    changePassword: 'Passwort ändern',
    close: 'Schließen',
    configAccountText:
      'Das Konto {name} ist in der Serverkonfiguration definiert, daher kann sein Passwort hier nicht geändert werden. Passwort-Hash in der Worker-Konfigurationsdatei ändern und den Worker neu starten.',
    userMenu: 'Benutzermenü',
    signedInAs: 'Angemeldet als',
    roleInfoAdmin: 'Administrator: voller Lese- und Schreibzugriff.',
    roleInfoViewer: 'Betrachter: nur Lesezugriff.',
    roleNameAdmin: 'Administrator',
    roleNameViewer: 'Betrachter',
    readOnlyAccount: 'Konto nur mit Lesezugriff',
  },
  vi: {
    currentPassword: 'Mật khẩu hiện tại',
    newPassword: 'Mật khẩu mới',
    repeatPassword: 'Nhập lại mật khẩu mới',
    mismatch: 'Hai mật khẩu không khớp.',
    passwordInvalid:
      'Dùng ít nhất 12 ký tự (tối đa 72 byte), khác tên đăng nhập.',
    passwordHint: 'Các phiên đăng nhập khác sẽ bị đăng xuất.',
    passwordRules: 'Quy tắc mật khẩu',
    ruleLength: 'Tối thiểu 12 ký tự, tối đa 72 byte',
    ruleMatch: 'Hai ô mật khẩu mới khớp nhau',
    forcedHint: 'Đặt mật khẩu mới trước khi vào console.',
    passwordStrength: 'Độ mạnh: {strength}',
    weak: 'Yếu',
    fair: 'Trung bình',
    strong: 'Mạnh',
    passwordChanged: 'Đã đổi mật khẩu.',
    changePasswordFailed: 'Không đổi được mật khẩu. Thử lại.',
    changingPassword: 'Đang đổi mật khẩu',
    loginTitle: 'Đăng nhập',
    errorInvalidCredentialsAttempts:
      'Tên đăng nhập và mật khẩu không khớp. Còn {n} lần thử trước khi bị khóa tạm thời.',
    signingIn: 'Đang đăng nhập',
    loginHelp: 'Quên mật khẩu hoặc đăng nhập lần đầu?',
    firstRunHint: 'Xem cách lấy mật khẩu quản trị.',
    username: 'Tên đăng nhập',
    password: 'Mật khẩu',
    keepSignedIn: 'Giữ đăng nhập',
    signIn: 'Đăng nhập',
    usernameRequired: 'Nhập tên đăng nhập.',
    passwordRequired: 'Nhập mật khẩu.',
    errorInvalidCredentials: 'Tên đăng nhập và mật khẩu không khớp.',
    errorRateLimited: 'Có quá nhiều lần đăng nhập. Thử lại sau {time}.',
    errorRateLimitedUnknown: 'Có quá nhiều lần đăng nhập. Thử lại sau.',
    errorUnknown: 'Chưa đăng nhập được. Kiểm tra kết nối rồi thử lại.',
    connectionUnavailable:
      'Không kết nối được server nên chưa kiểm tra được quyền truy cập. Thử lại để tiếp tục.',
    retryConnection: 'Thử lại',
    secondsShort: '{n} giây',
    minutesShort: '{n} phút',
    logout: 'Đăng xuất',
    changePassword: 'Đổi mật khẩu',
    close: 'Đóng',
    configAccountText:
      'Tài khoản {name} được định nghĩa trong cấu hình server nên không đổi mật khẩu tại đây được. Hãy cập nhật password hash trong tệp cấu hình worker rồi khởi động lại worker.',
    userMenu: 'Menu người dùng',
    signedInAs: 'Đăng nhập với tên',
    roleInfoAdmin: 'Quản trị viên: toàn quyền đọc và ghi.',
    roleInfoViewer: 'Người xem: chỉ đọc.',
    roleNameAdmin: 'Quản trị viên',
    roleNameViewer: 'Người xem',
    readOnlyAccount: 'tài khoản chỉ đọc',
  },
};
