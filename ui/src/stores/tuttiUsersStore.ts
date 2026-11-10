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
import { computed, reactive, toRefs } from 'vue';
import { UserHelper } from '@/utils/helpers/UserHelper';
import { TuttiService } from '@/services/TuttiService';
import type { S3User } from '@/utils/types/users';

interface TuttiUsersState {
  isLoading: boolean;
  hasError: boolean;
  isLoaded: boolean;
  users: S3User[];
  mainStorage: string;
  storages: string[];
  /** false when the worker cannot seal secrets (no master password) */
  dynamicEnabled: boolean;
}

function getInitialState(): TuttiUsersState {
  return {
    isLoading: false,
    hasError: false,
    isLoaded: false,
    users: [],
    mainStorage: '',
    storages: [],
    dynamicEnabled: true,
  };
}

export const useTuttiUsersStore = defineStore('tuttiUsers', () => {
  const state = reactive<TuttiUsersState>(getInitialState());

  const incompleteUsers = computed(() =>
    state.users.filter((user) => user.complete === false),
  );

  /** Users that have credentials on the main storage (can get access keys). */
  const usersWithMain = computed(() =>
    state.users.filter((user) =>
      (user.storages ?? []).some(
        (s) =>
          s.storage === state.mainStorage && UserHelper.modeOf(s) !== 'missing',
      ),
    ),
  );

  async function load(showLoading = true) {
    if (showLoading) state.isLoading = true;

    try {
      const res = await TuttiService.getUsers();

      state.users = res.users ?? [];
      state.mainStorage = res.mainStorage ?? '';
      state.storages = res.storages ?? [];
      state.dynamicEnabled = res.dynamicCredentialsEnabled !== false;
      state.hasError = false;
      state.isLoaded = true;
    } catch {
      state.hasError = true;
    } finally {
      state.isLoading = false;
    }
  }

  function find(name: string): S3User | undefined {
    return state.users.find((user) => user.name === name);
  }

  function $reset() {
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    incompleteUsers,
    usersWithMain,
    load,
    find,
    $reset,
  };
});
