/* Copyright © 2026 hieunyit. Licensed under AGPL-3.0. */
import management from './i18nManagement';

/**
 * Strings of the redesigned access pages (S3 users, proxy accounts, console
 * users), merged over the older management messages so existing keys keep
 * working. vi falls back to en for keys it does not define.
 */
const access = {
  en: {
    statusComplete: 'Ready',
    statusIncomplete: 'Incomplete',
    emptyUsersTitle: 'No S3 users yet',
    noMatchesTitle: 'No users match this filter',
    noMatchesHint: 'Clear the search or switch to All to see every user.',
    searchLabel: 'Search users',
    usersCount: '{count} users',
    startOver: 'Start over',
    stepKeysHint:
      'The secret is shown once after you create the user. Copy it before closing.',
    createdTitle: 'User {name} created',
    secretLabel: 'Secret access key',
    accessKeyIdLabel: 'Access key ID',
    secretSaved: 'I have saved the secret',
    openUser: 'Open user',
    createConsoleUser: 'Create console user',
    changeRole: 'Change role',
    saveRole: 'Save role',
    newPasswordTitle: 'One-time password',
    closeAfterSaving: 'Close',
    forbiddenTitle: 'Administrators only',
    forbiddenText: 'Console users can only be managed by an administrator.',
    emptyConsoleTitle: 'No console users',
    resetPasswordConfirm:
      'Reset the password of {name}? A new one-time password is generated and their sessions end.',
    disableConfirm: 'Disable {name}? They can no longer sign in.',
    enableConfirm: 'Enable {name}? They can sign in again.',
    deleteConsoleConfirm: 'Delete {name}? They can no longer sign in.',
    deleteLabel: 'Delete user',
    consoleSelfHint: 'You cannot change or delete your own account here.',
    lastAdminHint: 'The last enabled administrator cannot be changed.',
    clearFilters: 'Clear filters',
    createFailed: 'Could not create the user.',
  },
  de: {
    statusComplete: 'Bereit',
    statusIncomplete: 'Unvollständig',
    emptyUsersTitle: 'Noch keine S3-Benutzer',
    noMatchesTitle: 'Kein Benutzer passt zu diesem Filter',
    noMatchesHint:
      'Suche zurücksetzen oder auf Alle wechseln, um jeden Benutzer zu sehen.',
    searchLabel: 'Benutzer suchen',
    usersCount: '{count} Benutzer',
    startOver: 'Neu beginnen',
    stepKeysHint:
      'Das Secret wird nach dem Anlegen einmal angezeigt. Vor dem Schließen kopieren.',
    createdTitle: 'Benutzer {name} angelegt',
    secretLabel: 'Secret Access Key',
    accessKeyIdLabel: 'Access Key ID',
    secretSaved: 'Ich habe das Secret gespeichert',
    openUser: 'Benutzer öffnen',
    createConsoleUser: 'Konsolenbenutzer anlegen',
    changeRole: 'Rolle ändern',
    saveRole: 'Rolle speichern',
    newPasswordTitle: 'Einmal-Passwort',
    closeAfterSaving: 'Schließen',
    forbiddenTitle: 'Nur für Administratoren',
    forbiddenText:
      'Konsolenbenutzer können nur von Administratoren verwaltet werden.',
    emptyConsoleTitle: 'Keine Konsolenbenutzer',
    resetPasswordConfirm:
      'Passwort von {name} zurücksetzen? Ein neues Einmal-Passwort wird erzeugt, die Sitzungen enden.',
    disableConfirm:
      'Konto {name} deaktivieren? Anmeldung ist dann nicht mehr möglich.',
    enableConfirm: 'Konto {name} aktivieren? Anmeldung ist wieder möglich.',
    deleteConsoleConfirm:
      '{name} löschen? Anmeldung ist dann nicht mehr möglich.',
    deleteLabel: 'Benutzer löschen',
    consoleSelfHint:
      'Das eigene Konto kann hier nicht geändert oder gelöscht werden.',
    lastAdminHint:
      'Der letzte aktive Administrator kann nicht geändert werden.',
    clearFilters: 'Filter zurücksetzen',
    createFailed: 'Benutzer konnte nicht angelegt werden.',
  },
  vi: {
    statusComplete: 'Sẵn sàng',
    statusIncomplete: 'Chưa đủ',
    emptyUsersTitle: 'Chưa có S3 user nào',
    noMatchesTitle: 'Không có user khớp bộ lọc',
    noMatchesHint: 'Xóa ô tìm kiếm hoặc chọn Tất cả để xem mọi user.',
    searchLabel: 'Tìm user',
    usersCount: '{count} user',
    startOver: 'Làm lại từ đầu',
    stepKeysHint:
      'Secret chỉ hiện một lần sau khi tạo user. Hãy sao chép trước khi đóng.',
    createdTitle: 'Đã tạo user {name}',
    secretLabel: 'Secret access key',
    accessKeyIdLabel: 'Access key ID',
    secretSaved: 'Tôi đã lưu secret',
    openUser: 'Mở user',
    createConsoleUser: 'Tạo console user',
    changeRole: 'Đổi role',
    saveRole: 'Lưu role',
    newPasswordTitle: 'Mật khẩu dùng một lần',
    closeAfterSaving: 'Đóng',
    forbiddenTitle: 'Chỉ dành cho quản trị viên',
    forbiddenText: 'Chỉ quản trị viên mới quản lý được console user.',
    emptyConsoleTitle: 'Chưa có console user',
    resetPasswordConfirm:
      'Đặt lại mật khẩu của {name}? Hệ thống tạo mật khẩu một lần mới và các phiên đăng nhập kết thúc.',
    disableConfirm: 'Tắt {name}? Họ không đăng nhập được nữa.',
    enableConfirm: 'Bật {name}? Họ đăng nhập lại được.',
    deleteConsoleConfirm: 'Xóa {name}? Họ không đăng nhập được nữa.',
    deleteLabel: 'Xóa user',
    consoleSelfHint:
      'Bạn không thể đổi hoặc xóa tài khoản của chính mình ở đây.',
    lastAdminHint: 'Không thể đổi quản trị viên đang bật cuối cùng.',
    clearFilters: 'Xóa lọc',
    createFailed: 'Không tạo được user.',
  },
};

export default {
  en: { ...management.en, ...access.en },
  de: { ...management.de, ...access.de },
  vi: { ...management.en, ...access.vi },
};
