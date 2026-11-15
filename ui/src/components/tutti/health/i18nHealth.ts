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
// Plain locale codes (en, de, vi) keep this file free of the UI kit bundle.
export default <Record<string, Record<string, string>>>{
  en: {
    unavailable: 'Unavailable',
    warning: 'Available, with errors',
    failures: '{count} consecutive failures',
    since: 'since {when}',
  },
  de: {
    unavailable: 'Nicht verfügbar',
    warning: 'Verfügbar, mit Fehlern',
    failures: '{count} Fehler in Folge',
    since: 'seit {when}',
  },
  vi: {
    unavailable: 'Không khả dụng',
    warning: 'Sẵn sàng, có lỗi',
    failures: '{count} lần lỗi liên tiếp',
    since: 'từ {when}',
  },
};
