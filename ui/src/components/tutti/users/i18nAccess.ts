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
    noCredentialsTitle: 'No storage credentials',
    noCredentialsText:
      'No storage credentials or protection group access are assigned to this user.',
    verifyAndSave: 'Verify and save',
    scope: 'Scope',
    scopeStorage: 'Storage-level',
    scopeBuckets: 'Bucket-level',
    scopeUser: 'User-level',
    primaryStorage: 'Primary storage',
    coveredBuckets: 'Covered buckets',
    policyHint:
      'The access policy sets which operations this user may perform on which buckets.',
    allBucketsNote: '(all current and future buckets)',
    coverageTitle: 'Protection group coverage',
    coverageHint:
      'Requests through the proxy are protected and replicated by the protection groups covering these buckets.',
    noGroupsTitle: 'No protection groups cover this user',
    userNotFoundHint: 'The user may have been deleted.',
    backToUsers: 'Back to users',
    dangerZone: 'Danger zone',
    dangerZoneHint: 'Deleting a user removes its credentials and access keys.',
    deleteUser: 'Delete user',
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
    noCredentialsTitle: 'Keine Speicher-Zugangsdaten',
    noCredentialsText:
      'Diesem Benutzer sind keine Speicher-Zugangsdaten oder Schutzgruppen-Zugriffe zugewiesen.',
    verifyAndSave: 'Prüfen und speichern',
    scope: 'Geltungsbereich',
    scopeStorage: 'Speicherebene',
    scopeBuckets: 'Bucket-Ebene',
    scopeUser: 'Benutzerebene',
    primaryStorage: 'Primärer Speicher',
    coveredBuckets: 'Abgedeckte Buckets',
    policyHint:
      'Die Zugriffsrichtlinie legt fest, welche Operationen dieser Benutzer auf welchen Buckets ausführen darf.',
    allBucketsNote: '(alle aktuellen und künftigen Buckets)',
    coverageTitle: 'Schutzgruppen-Abdeckung',
    coverageHint:
      'Anfragen über den Proxy werden von den Schutzgruppen dieser Buckets geschützt und repliziert.',
    noGroupsTitle: 'Keine Schutzgruppe deckt diesen Benutzer ab',
    userNotFoundHint: 'Der Benutzer wurde möglicherweise gelöscht.',
    backToUsers: 'Zurück zu Benutzern',
    dangerZone: 'Gefahrenbereich',
    dangerZoneHint:
      'Beim Löschen werden Zugangsdaten und Zugriffsschlüssel des Benutzers entfernt.',
    deleteUser: 'Benutzer löschen',
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
    noCredentialsTitle: 'Chưa có credentials storage',
    noCredentialsText:
      'User này chưa được gán credentials storage hoặc quyền theo nhóm bảo vệ.',
    verifyAndSave: 'Kiểm tra và lưu',
    scope: 'Phạm vi',
    scopeStorage: 'Cấp storage',
    scopeBuckets: 'Cấp bucket',
    scopeUser: 'Cấp user',
    primaryStorage: 'Storage chính',
    coveredBuckets: 'Bucket được bảo vệ',
    policyHint: 'Access policy quy định user được thao tác gì trên bucket nào.',
    allBucketsNote: '(mọi bucket hiện tại và tương lai)',
    coverageTitle: 'Nhóm bảo vệ áp dụng',
    coverageHint:
      'Request qua proxy được bảo vệ và nhân bản theo nhóm bảo vệ của các bucket này.',
    noGroupsTitle: 'Chưa có nhóm bảo vệ nào áp dụng',
    userNotFoundHint: 'User có thể đã bị xóa.',
    backToUsers: 'Về danh sách user',
    dangerZone: 'Vùng nguy hiểm',
    dangerZoneHint: 'Xóa user sẽ gỡ credentials và access key của user đó.',
    deleteUser: 'Xóa user',
    clearFilters: 'Xóa lọc',
    createFailed: 'Không tạo được user.',
  },
};

export default {
  en: { ...management.en, ...access.en },
  de: { ...management.de, ...access.de },
  vi: { ...management.en, ...access.vi },
};
