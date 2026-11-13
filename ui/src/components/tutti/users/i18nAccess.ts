/* Copyright © 2026 hieunyit. Licensed under AGPL-3.0. */
import management from './i18nManagement';

/**
 * Strings of the redesigned access pages (S3 users, proxy accounts, console
 * users), merged over the older management messages so existing keys keep
 * working. vi falls back to en for keys it does not define.
 */
const access = {
  en: {
    storageCredentialsTitle: 'Storage credentials',
    storageCredentialsHint:
      'These storages have no connection account. Supply existing S3 credentials for this user, or configure a connection account on the storage. All buckets is an access policy; it does not create storage credentials.',
    credentialsForStorage: 'Credentials for {storage}',
    loadingStorageAccounts: 'Loading storage accounts…',
    storageAccountsLoadFailed:
      'Storage accounts could not be loaded. Retry before creating the user.',
    retryStorageAccounts: 'Retry',
    verifyStorageCredentials:
      'Verify these credentials before creating the user',
    completeIdentityAndCredentials:
      'Complete the user name, storage credentials and permissions before creating the user.',
    statusComplete: 'Ready',
    addPattern: 'Add',
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
    storageCredentialsTitle: 'Storage-Zugangsdaten',
    storageCredentialsHint:
      'Für diese Storages fehlt ein Verbindungskonto. Bestehende S3-Zugangsdaten für den Benutzer eingeben oder ein Verbindungskonto konfigurieren. Alle Buckets ist eine Zugriffsrichtlinie und erzeugt keine Zugangsdaten.',
    credentialsForStorage: 'Zugangsdaten für {storage}',
    loadingStorageAccounts: 'Storage-Konten werden geladen…',
    storageAccountsLoadFailed:
      'Storage-Konten konnten nicht geladen werden. Vor dem Anlegen erneut versuchen.',
    retryStorageAccounts: 'Erneut versuchen',
    verifyStorageCredentials: 'Diese Zugangsdaten vor dem Anlegen prüfen',
    completeIdentityAndCredentials:
      'Benutzernamen, Storage-Zugangsdaten und Berechtigungen vervollständigen.',
    statusComplete: 'Bereit',
    addPattern: 'Hinzufügen',
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
    storageCredentialsTitle: 'Credentials của storage',
    storageCredentialsHint:
      'Các storage này chưa có connection account. Nhập S3 credentials có sẵn cho user, hoặc cấu hình connection account tại storage. All buckets là quyền truy cập, không tự tạo credentials của storage.',
    credentialsForStorage: 'Credentials cho {storage}',
    loadingStorageAccounts: 'Đang tải tài khoản kết nối storage…',
    storageAccountsLoadFailed:
      'Không tải được tài khoản storage. Hãy thử lại trước khi tạo user.',
    retryStorageAccounts: 'Thử lại',
    verifyStorageCredentials: 'Kiểm tra credentials trước khi tạo user',
    completeIdentityAndCredentials:
      'Hoàn tất tên user, credentials storage và quyền truy cập trước khi tạo user.',
    statusComplete: 'Sẵn sàng',
    addPattern: 'Thêm',
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
