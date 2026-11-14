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

import type { NavigationGuard } from 'vue-router';
import { useAuthStore } from '@/stores/authStore';
import { RouteName } from '@/utils/types/router';

const authGuard: NavigationGuard = async (to) => {
  const auth = useAuthStore();

  await auth.init();

  if (to.name === RouteName.LOGIN) {
    // nothing to log into, or already logged in
    return auth.isAuthenticated ? { name: RouteName.TUTTI_HOME } : true;
  }

  if (!auth.isAuthenticated) {
    return {
      name: RouteName.LOGIN,
      query: to.fullPath === '/' ? {} : { redirect: to.fullPath },
    };
  }

  if (auth.mustChangePassword && auth.authEnabled) {
    if (to.name === RouteName.TUTTI_CHANGE_PASSWORD) return true;

    return {
      name: RouteName.TUTTI_CHANGE_PASSWORD,
      query: to.fullPath === '/' ? {} : { redirect: to.fullPath },
    };
  }

  if (to.name === RouteName.TUTTI_CHANGE_PASSWORD) {
    return { name: RouteName.TUTTI_HOME };
  }

  if (to.meta.requiresAdmin && !auth.isAdmin) {
    return { name: RouteName.TUTTI_HOME };
  }

  if (to.meta.requiresWrite && !auth.canWrite) {
    return { name: RouteName.TUTTI_HOME };
  }

  return true;
};

export default authGuard;
