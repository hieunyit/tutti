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

import { defineStore } from 'pinia';
import { computed, reactive, toRefs, watch } from 'vue';
import { helpers } from '@vuelidate/validators';
import useVuelidate from '@vuelidate/core';
import { useStoreI18n } from '@/i18n/useStoreI18n';
import type { TuttiStorage } from '@/utils/types/tuttiCore';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import i18nAddDiffReport from '@/components/tutti/add-diff-report/i18nAddDiffReport';
import {
  hasNoAdjacentPeriods,
  hasValidChars,
  hasValidPrefixSuffix,
  hasValidStartEnd,
  isNotIpAddress,
  isRequired,
  isValidLength,
} from '@/utils/validators/s3BucketNameValidator';

interface TuttiAddDiffReportState {
  isLoading: boolean;
  hasError: boolean;

  storages: TuttiStorage[];
  fromStorage: TuttiStorage | null;
  fromBucketName: string;
  toStorage: TuttiStorage | null;
  toBucketName: string;

  selectedUser: string | null;

  checkOnlyLastVersions: boolean;
  ignoreEtags: boolean;
  ignoreSizes: boolean;

  isSubmitting: boolean;
}

function getInitialState(): TuttiAddDiffReportState {
  return {
    isLoading: false,
    hasError: false,

    storages: [],
    fromStorage: null,
    fromBucketName: '',
    toStorage: null,
    toBucketName: '',

    selectedUser: null,

    checkOnlyLastVersions: true,
    ignoreEtags: false,
    ignoreSizes: false,

    isSubmitting: false,
  };
}

export const useTuttiAddDiffReportStore = defineStore(
  'tuttiAddDiffReport',
  () => {
    const state = reactive<TuttiAddDiffReportState>(getInitialState());
    const { t } = useStoreI18n({
      messages: i18nAddDiffReport,
    });
    const hasEnoughStorages = computed(() => state.storages.length >= 2);

    // Filter for users available on both storages
    const users = computed<string[]>(() => {
      if (!state.fromStorage || !state.toStorage) return [];

      const fromStorageAliases = new Set(
        state.fromStorage.credentials.map((credential) => credential.alias),
      );

      return state.toStorage.credentials
        .map((credential) => credential.alias)
        .filter((alias) => fromStorageAliases.has(alias))
        .sort();
    });

    function bucketNameValidationRules() {
      return {
        required: helpers.withMessage(t('bucketRequired'), isRequired),
        validLength: helpers.withMessage(t('bucketErrLength'), isValidLength),
        validChars: helpers.withMessage(t('bucketErrChars'), hasValidChars),
        validStartEnd: helpers.withMessage(
          t('bucketErrStartEnd'),
          hasValidStartEnd,
        ),
        noAdjacentPeriods: helpers.withMessage(
          t('bucketErrAdjacentPeriods'),
          hasNoAdjacentPeriods,
        ),
        notIpAddress: helpers.withMessage(
          t('bucketErrIpAddress'),
          isNotIpAddress,
        ),
        validPrefixSuffix: helpers.withMessage(
          t('bucketErrPrefixSuffix'),
          hasValidPrefixSuffix,
        ),
      };
    }

    const validationRules = computed(() => ({
      fromStorage: {
        required: helpers.withMessage(
          t('storageRequired'),
          (value: TuttiStorage | null) => !!value,
        ),
      },
      fromBucketName: bucketNameValidationRules(),
      toStorage: {
        required: helpers.withMessage(
          t('storageRequired'),
          (value: TuttiStorage | null) => !!value,
        ),
      },
      toBucketName: bucketNameValidationRules(),
      selectedUser: {
        required: helpers.withMessage(
          t('userRequired'),
          (value: string | null) => !!value,
        ),
      },
    }));

    const validator = useVuelidate(validationRules, state);

    async function initAddDiffReportPage() {
      state.isLoading = true;
      state.hasError = false;

      try {
        const { storages } = await TuttiCoreService.getStorages();

        state.storages = storages;
        prepareForm();
      } catch {
        state.hasError = true;
      } finally {
        state.isLoading = false;
      }
    }

    /** the only other storage is the obvious destination; with more, the user picks */
    function defaultDestination(source: TuttiStorage): TuttiStorage | null {
      const others = state.storages.filter(
        (storage) => storage.name !== source.name,
      );

      return others.length === 1 ? (others[0] ?? null) : null;
    }

    // a source that equals the destination clears the destination
    watch(
      () => state.fromStorage,
      (newFromStorage) => {
        if (
          newFromStorage &&
          state.toStorage &&
          state.toStorage.name === newFromStorage.name
        ) {
          state.toStorage = defaultDestination(newFromStorage);
        }
      },
    );

    // keep the user valid when the storages change
    watch(users, (newUsers) => {
      if (state.selectedUser && newUsers.includes(state.selectedUser)) return;

      state.selectedUser = newUsers[0] ?? null;
    });

    function prepareForm() {
      const mainStorage =
        state.storages.find((storage) => storage.isMain) ??
        state.storages[0] ??
        null;

      state.fromStorage = mainStorage;
      state.toStorage = mainStorage ? defaultDestination(mainStorage) : null;
      state.selectedUser = users.value[0] ?? null;
    }

    async function createDiffReport() {
      const { fromStorage, toStorage, selectedUser } = state;

      if (!fromStorage || !toStorage || !selectedUser) return;

      state.isSubmitting = true;

      try {
        await TuttiCoreService.addDiffReport({
          locations: [
            { storage: fromStorage.name, bucket: state.fromBucketName },
            { storage: toStorage.name, bucket: state.toBucketName },
          ],
          user: selectedUser,
          checkOnlyLastVersions: state.checkOnlyLastVersions,
          ignoreEtags: state.ignoreEtags,
          ignoreSizes: state.ignoreSizes,
        });
      } finally {
        state.isSubmitting = false;
      }
    }

    function $reset() {
      Object.assign(state, getInitialState());
      validator.value.$reset();
    }

    return {
      ...toRefs(state),
      hasEnoughStorages,
      users,
      initAddDiffReportPage,
      validator,
      createDiffReport,
      $reset,
    };
  },
);
