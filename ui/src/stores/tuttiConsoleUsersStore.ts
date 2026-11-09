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

import { defineStore } from 'pinia';
import { reactive, toRefs } from 'vue';
import axios from 'axios';
import { TuttiService } from '@/services/TuttiService';
import type { ConsoleUser } from '@/utils/types/users';

interface ConsoleUsersState {
  isLoading: boolean;
  hasError: boolean;
  /** sign-in disabled on the server (FailedPrecondition) */
  authDisabled: boolean;
  users: ConsoleUser[];
}

function getInitialState(): ConsoleUsersState {
  return { isLoading: false, hasError: false, authDisabled: false, users: [] };
}

export const useTuttiConsoleUsersStore = defineStore(
  'tuttiConsoleUsers',
  () => {
    const state = reactive<ConsoleUsersState>(getInitialState());

    async function load(showLoading = true) {
      if (showLoading) state.isLoading = true;

      try {
        const res = await TuttiService.getConsoleUsers();

        state.users = res.users ?? [];
        state.hasError = false;
        state.authDisabled = false;
      } catch (e) {
        // FailedPrecondition: "sign-in is disabled"
        const code = axios.isAxiosError(e)
          ? (e.response?.data as { code?: number } | undefined)?.code
          : undefined;

        state.authDisabled = code === 9;
        state.hasError = code !== 9;
      } finally {
        state.isLoading = false;
      }
    }

    function $reset() {
      Object.assign(state, getInitialState());
    }

    return { ...toRefs(state), load, $reset };
  },
);
