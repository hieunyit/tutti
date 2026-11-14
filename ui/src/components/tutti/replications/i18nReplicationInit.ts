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
    initAtLeast: 'at least',
    initEtaLeft: '~{eta} left',
    initEtaUnknown: 'time left unknown',
    initSpeedUnknown: 'speed unknown',
    initObjectsCount: '{done} / {total} objects',
  },
  [I18nLocale.DE]: {
    initBytesLabel: 'Erste Replikation',
    initAtLeast: 'mindestens',
    initEtaLeft: 'noch ~{eta}',
    initEtaUnknown: 'Restzeit unbekannt',
    initSpeedUnknown: 'Geschwindigkeit unbekannt',
    initObjectsCount: '{done} / {total} Objekte',
  },
  vi: {
    initBytesLabel: 'Sao chép ban đầu',
    initAtLeast: 'tối thiểu',
    initEtaLeft: 'còn ~{eta}',
    initEtaUnknown: 'chưa rõ thời gian còn lại',
    initSpeedUnknown: 'chưa rõ tốc độ',
    initObjectsCount: '{done} / {total} object',
  },
};
