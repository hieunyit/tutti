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
    initBytesLabel: 'Initial Replication',
    initCopied: 'Copied {size} (incl. in progress)',
    initObjectsCount: '{done} / {total} objects',
  },
  [I18nLocale.DE]: {
    initBytesLabel: 'Erste Replikation',
    initCopied: '{size} kopiert (inkl. laufender)',
    initObjectsCount: '{done} / {total} Objekte',
  },
  vi: {
    initBytesLabel: 'Sao chép ban đầu',
    initCopied: 'Đã sao chép {size} (gồm đang chạy)',
    initObjectsCount: '{done} / {total} object',
  },
};
