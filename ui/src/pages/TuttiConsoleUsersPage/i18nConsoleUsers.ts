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

export default {
  en: {
    title: 'Console users',
    create: 'Create console user',
    searchLabel: 'Search console users',
    username: 'Username',
    role: 'Role',
    status: 'Status',
    lastLogin: 'Last sign-in',
    admin: 'Administrator',
    viewer: 'Viewer',
    adminHint: 'Can change settings and manage everything.',
    viewerHint: 'Read-only access.',
    active: 'Active',
    invited: 'Invited',
    disabled: 'Disabled',
    you: 'You',
    config: 'Config',
    configHint: 'Defined in the server configuration. Change it there.',
    rowActions: 'Actions for {name}',
    changeRole: 'Change role',
    saveRole: 'Save role',
    resetPassword: 'Reset password',
    enable: 'Enable',
    disable: 'Disable',
    delete: 'Delete',
    deleteLabel: 'Delete console user',
    cancel: 'Cancel',
    generate: 'Generate a one-time password',
    generateHint:
      'Shown once after you create the user, who must change it at first sign-in.',
    custom: 'Set a password',
    passwordLabel: 'Password',
    passwordHint: 'At least 12 characters.',
    usernameInvalid:
      'Use up to 64 letters, digits, dots, dashes, underscores or @, starting with a letter or digit.',
    passwordInvalid:
      'Use at least 12 characters (72 bytes at most), different from the username.',
    lastAdmin: 'Keep at least one enabled administrator.',
    resetConfirm:
      'A new one-time password is generated and the sessions of {name} end.',
    disableConfirm: '{name} can no longer sign in.',
    enableConfirm: '{name} can sign in again.',
    deleteConfirm: '{name} can no longer sign in. This cannot be undone.',
    newPasswordTitle: 'One-time password',
    close: 'Close',
    emptyTitle: 'No console users',
    noMatchTitle: 'No console users match',
    clearSearch: 'Clear search',
    forbiddenTitle: 'Administrators only',
    forbiddenText: 'Only administrators can manage console users.',
    authDisabled:
      'Sign-in is turned off. Turn on authentication to manage console users.',
    error: 'The change could not be saved. Try again.',
  },
  de: {
    title: 'Konsolenbenutzer',
    create: 'Konsolenbenutzer anlegen',
    searchLabel: 'Konsolenbenutzer suchen',
    username: 'Benutzername',
    role: 'Rolle',
    status: 'Status',
    lastLogin: 'Letzte Anmeldung',
    admin: 'Administrator',
    viewer: 'Betrachter',
    adminHint: 'Darf Einstellungen ändern und alles verwalten.',
    viewerHint: 'Nur Lesezugriff.',
    active: 'Aktiv',
    invited: 'Eingeladen',
    disabled: 'Deaktiviert',
    you: 'Sie',
    config: 'Konfiguration',
    configHint: 'In der Serverkonfiguration definiert. Dort ändern.',
    rowActions: 'Aktionen für {name}',
    changeRole: 'Rolle ändern',
    saveRole: 'Rolle speichern',
    resetPassword: 'Passwort zurücksetzen',
    enable: 'Aktivieren',
    disable: 'Deaktivieren',
    delete: 'Löschen',
    deleteLabel: 'Konsolenbenutzer löschen',
    cancel: 'Abbrechen',
    generate: 'Einmalpasswort erzeugen',
    generateHint:
      'Wird nach dem Anlegen einmal angezeigt. Der Benutzer muss es bei der ersten Anmeldung ändern.',
    custom: 'Passwort festlegen',
    passwordLabel: 'Passwort',
    passwordHint: 'Mindestens 12 Zeichen.',
    usernameInvalid:
      'Bis zu 64 Buchstaben, Ziffern, Punkte, Bindestriche, Unterstriche oder @, beginnend mit Buchstabe oder Ziffer.',
    passwordInvalid:
      'Mindestens 12 Zeichen (höchstens 72 Bytes), anders als der Benutzername.',
    lastAdmin: 'Mindestens einen aktiven Administrator beibehalten.',
    resetConfirm:
      'Ein neues Einmalpasswort wird erzeugt und die Sitzungen von {name} enden.',
    disableConfirm: '{name} kann sich nicht mehr anmelden.',
    enableConfirm: '{name} kann sich wieder anmelden.',
    deleteConfirm:
      '{name} kann sich nicht mehr anmelden. Das lässt sich nicht rückgängig machen.',
    newPasswordTitle: 'Einmalpasswort',
    close: 'Schließen',
    emptyTitle: 'Keine Konsolenbenutzer',
    noMatchTitle: 'Keine Konsolenbenutzer gefunden',
    clearSearch: 'Suche zurücksetzen',
    forbiddenTitle: 'Nur für Administratoren',
    forbiddenText: 'Nur Administratoren können Konsolenbenutzer verwalten.',
    authDisabled:
      'Die Anmeldung ist ausgeschaltet. Authentifizierung einschalten, um Konsolenbenutzer zu verwalten.',
    error:
      'Die Änderung konnte nicht gespeichert werden. Versuchen Sie es erneut.',
  },
  vi: {
    title: 'Người dùng console',
    create: 'Tạo người dùng console',
    searchLabel: 'Tìm người dùng console',
    username: 'Tên đăng nhập',
    role: 'Vai trò',
    status: 'Trạng thái',
    lastLogin: 'Đăng nhập gần nhất',
    admin: 'Quản trị viên',
    viewer: 'Người xem',
    adminHint: 'Được đổi cài đặt và quản lý mọi thứ.',
    viewerHint: 'Chỉ đọc.',
    active: 'Đang hoạt động',
    invited: 'Đã mời',
    disabled: 'Đã tắt',
    you: 'Bạn',
    config: 'Cấu hình',
    configHint: 'Được định nghĩa trong cấu hình server. Hãy sửa ở đó.',
    rowActions: 'Thao tác cho {name}',
    changeRole: 'Đổi vai trò',
    saveRole: 'Lưu vai trò',
    resetPassword: 'Đặt lại mật khẩu',
    enable: 'Bật',
    disable: 'Tắt',
    delete: 'Xóa',
    deleteLabel: 'Xóa người dùng console',
    cancel: 'Hủy',
    generate: 'Tạo mật khẩu dùng một lần',
    generateHint:
      'Hiện một lần sau khi tạo. Người dùng phải đổi mật khẩu ở lần đăng nhập đầu.',
    custom: 'Tự đặt mật khẩu',
    passwordLabel: 'Mật khẩu',
    passwordHint: 'Ít nhất 12 ký tự.',
    usernameInvalid:
      'Tối đa 64 chữ cái, chữ số, dấu chấm, gạch ngang, gạch dưới hoặc @, bắt đầu bằng chữ cái hoặc chữ số.',
    passwordInvalid:
      'Dùng ít nhất 12 ký tự (tối đa 72 byte), khác tên đăng nhập.',
    lastAdmin: 'Phải giữ ít nhất một quản trị viên đang bật.',
    resetConfirm:
      'Mật khẩu dùng một lần mới sẽ được tạo và các phiên của {name} kết thúc.',
    disableConfirm: '{name} sẽ không đăng nhập được nữa.',
    enableConfirm: '{name} có thể đăng nhập lại.',
    deleteConfirm: '{name} sẽ không đăng nhập được nữa. Không thể hoàn tác.',
    newPasswordTitle: 'Mật khẩu dùng một lần',
    close: 'Đóng',
    emptyTitle: 'Chưa có người dùng console',
    noMatchTitle: 'Không có người dùng console nào khớp',
    clearSearch: 'Xóa tìm kiếm',
    forbiddenTitle: 'Chỉ dành cho quản trị viên',
    forbiddenText: 'Chỉ quản trị viên mới quản lý được người dùng console.',
    authDisabled:
      'Đăng nhập đang tắt. Hãy bật xác thực để quản lý người dùng console.',
    error: 'Chưa lưu được thay đổi. Hãy thử lại.',
  },
};
