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
    clearFilters: 'Xóa lọc',
    createFailed: 'Không tạo được user.',
  },
};

export default {
  en: { ...management.en, ...access.en },
  de: { ...management.de, ...access.de },
  vi: { ...management.en, ...access.vi },
};
