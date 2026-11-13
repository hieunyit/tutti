/*
 * Copyright © 2026 Clyso GmbH
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
    mainStorage: 'Main',
    followerStorage: 'Follower',
    address: 'Address',
    allGood: 'all good',
    downAt: 'down at',
    errorTitle: 'Error',
    successTitle: 'Success',
    errorAction: 'Retry',
    direction: 'Direction',
    user: 'User',
    buckets: 'Buckets',
  },
  [I18nLocale.DE]: {
    mainStorage: 'Hauptspeicher', // Consistent with your suggestion
    followerStorage: 'Follower',
    address: 'Adresse',
    allGood: 'Alles in Ordnung',
    downAt: 'Abgesenkt um',
    errorTitle: 'Fehler',
    successTitle: 'Erfolg',
    errorAction: 'Wiederholen',
    direction: 'Richtung',
    user: 'Benutzer',
    buckets: 'Buckets',
  },
  vi: {
    mainStorage: 'Chính',
    followerStorage: 'Follower',
    address: 'Địa chỉ',
    allGood: 'mọi thứ ổn',
    downAt: 'ngừng hoạt động lúc',
    errorTitle: 'Lỗi',
    successTitle: 'Thành công',
    errorAction: 'Thử lại',
    direction: 'Hướng',
    user: 'User',
    buckets: 'Bucket',
  },
};
