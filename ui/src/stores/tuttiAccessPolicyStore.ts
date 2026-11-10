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
import { TuttiService } from '@/services/TuttiService';
import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
import type {
  TuttiAccessPolicyForm,
  TuttiDynamicKeyStatus,
  TuttiProxyAccount,
  TuttiProxyAccountSecret,
} from '@/utils/types/tutti';

interface TuttiAccessPolicyState {
  isLoading: boolean;
  hasError: boolean;
  accounts: TuttiProxyAccount[];
  /** false when the worker cannot create managed keys */
  dynamicAvailable: boolean;
  dynamicReason: string;
  /** address of a proxy, used for the .env download; empty when unknown */
  proxyAddress: string;
  isSaving: boolean;
  /** backend validation message, or 'unknown' */
  saveError: string | null;
}

function getInitialState(): TuttiAccessPolicyState {
  return {
    isLoading: false,
    hasError: false,
    accounts: [],
    dynamicAvailable: true,
    dynamicReason: '',
    proxyAddress: '',
    isSaving: false,
    saveError: null,
  };
}

export const useTuttiAccessPolicyStore = defineStore(
  'tuttiAccessPolicy',
  () => {
    const state = reactive<TuttiAccessPolicyState>(getInitialState());

    async function reloadAccounts() {
      const res = await TuttiService.getProxyAccounts();

      state.accounts = res.accounts ?? [];
      // older workers do not report it: assume available
      state.dynamicAvailable = res.dynamicAccounts?.available !== false;
      state.dynamicReason = res.dynamicAccounts?.reason ?? '';
    }

    /** Best effort: the .env download needs the proxy address. */
    async function loadProxyAddress() {
      try {
        const res = await TuttiService.getProxies();

        state.proxyAddress =
          (res.proxies ?? []).find((proxy) => proxy.address)?.address ?? '';
      } catch {
        state.proxyAddress = '';
      }
    }

    async function init() {
      state.isLoading = true;
      state.hasError = false;

      try {
        await Promise.all([reloadAccounts(), loadProxyAddress()]);
      } catch {
        state.hasError = true;
      } finally {
        state.isLoading = false;
      }
    }

    /** Returns true on success; on failure sets saveError. */
    async function savePolicy(form: TuttiAccessPolicyForm): Promise<boolean> {
      state.isSaving = true;
      state.saveError = null;

      try {
        await TuttiService.setAccessPolicy(
          AccessPolicyHelper.formToPolicy(form),
        );
        await reloadAccounts();

        return true;
      } catch (e) {
        state.saveError =
          AccessPolicyHelper.getValidationMessage(e) ?? 'unknown';

        return false;
      } finally {
        state.isSaving = false;
      }
    }

    async function removePolicy(user: string): Promise<boolean> {
      try {
        await TuttiService.deleteAccessPolicy(user);
        await reloadAccounts();

        return true;
      } catch (e) {
        // e.g. FailedPrecondition: the last policy of a shared-account user
        state.saveError =
          AccessPolicyHelper.getValidationMessage(e) ?? 'unknown';

        return false;
      }
    }

    /*
     * Dynamic key management. Secrets are only returned to the caller and are
     * never kept in the store state.
     */
    async function createAccount(
      user: string,
      description: string,
    ): Promise<TuttiProxyAccountSecret> {
      const secret = await TuttiService.createProxyAccount(
        user,
        description.trim(),
      );

      await reloadAccounts();

      return secret;
    }

    async function rotateSecret(
      accessKeyId: string,
      gracePeriod: string,
    ): Promise<TuttiProxyAccountSecret> {
      const secret = await TuttiService.rotateProxyAccountSecret(
        accessKeyId,
        gracePeriod,
      );

      await reloadAccounts();

      return secret;
    }

    async function setKeyStatus(
      accessKeyId: string,
      status: TuttiDynamicKeyStatus,
    ) {
      await TuttiService.setProxyAccountStatus(accessKeyId, status);
      await reloadAccounts();
    }

    async function deleteKey(accessKeyId: string) {
      await TuttiService.deleteProxyAccount(accessKeyId);
      await reloadAccounts();
    }

    function $reset() {
      Object.assign(state, getInitialState());
    }

    return {
      ...toRefs(state),
      init,
      savePolicy,
      removePolicy,
      createAccount,
      rotateSecret,
      setKeyStatus,
      deleteKey,
      $reset,
    };
  },
);
