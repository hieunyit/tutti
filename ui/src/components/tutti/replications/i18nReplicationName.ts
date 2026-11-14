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
    nameLabel: 'Name (optional)',
    namePlaceholder: 'e.g. prod to dr',
    nameHint:
      'A label to tell replications apart. It does not change what is copied.',
    nameColumn: 'Name',
    allBuckets: 'all buckets',
    nameTooLong: 'The name can have at most 64 characters.',
    nameControlChars: 'The name must not contain control characters.',
    renameAction: 'Rename',
    renameLabel: 'Replication name',
    renameSave: 'Save name',
    renameCancel: 'Cancel',
    renameFailed: 'Could not rename the replication. Try again.',
    renameHint: 'Enter saves, Escape cancels. Leave empty to remove the name.',
  },
  [I18nLocale.DE]: {
    nameLabel: 'Name (optional)',
    namePlaceholder: 'z. B. prod nach dr',
    nameHint:
      'Eine Bezeichnung, um Replikationen zu unterscheiden. Sie ändert nicht, was kopiert wird.',
    nameColumn: 'Name',
    allBuckets: 'alle Buckets',
    nameTooLong: 'Der Name darf höchstens 64 Zeichen lang sein.',
    nameControlChars: 'Der Name darf keine Steuerzeichen enthalten.',
    renameAction: 'Umbenennen',
    renameLabel: 'Name der Replikation',
    renameSave: 'Namen speichern',
    renameCancel: 'Abbrechen',
    renameFailed:
      'Die Replikation konnte nicht umbenannt werden. Bitte erneut versuchen.',
    renameHint:
      'Enter speichert, Escape bricht ab. Leer lassen, um den Namen zu entfernen.',
  },
  vi: {
    nameLabel: 'Tên (không bắt buộc)',
    namePlaceholder: 'ví dụ: prod sang dr',
    nameHint:
      'Nhãn để phân biệt các replication. Không thay đổi dữ liệu được sao chép.',
    nameColumn: 'Tên',
    allBuckets: 'tất cả bucket',
    nameTooLong: 'Tên tối đa 64 ký tự.',
    nameControlChars: 'Tên không được chứa ký tự điều khiển.',
    renameAction: 'Đổi tên',
    renameLabel: 'Tên replication',
    renameSave: 'Lưu tên',
    renameCancel: 'Hủy',
    renameFailed: 'Không thể đổi tên replication. Hãy thử lại.',
    renameHint: 'Enter để lưu, Escape để hủy. Để trống để xóa tên.',
  },
};
