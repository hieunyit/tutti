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

import { computed, inject, ref, type InjectionKey } from 'vue';
import { clearWizardDraft, useWizardDraft } from '@/components/kit';
import { TuttiService } from '@/services/TuttiService';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import { UserHelper } from '@/utils/helpers/UserHelper';
import {
  availableGroupBuckets as groupBucketItems,
  nextGroupBuckets,
  storageScopeGroups,
  storagesToQuery,
  type GrantMode,
  type GroupBucketItem,
} from '@/components/tutti/accounts/policyBuckets';
import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
import {
  BucketDiscoveryHelper,
  type StorageBucketEntry,
} from '@/utils/helpers/BucketDiscoveryHelper';
import type { TuttiProxyAccountSecret } from '@/utils/types/tutti';
import type { S3UserCredentialInput } from '@/utils/types/users';

export const USER_CREATE_DRAFT_KEY = 'user-create';

export type { GrantMode, GroupBucketItem };

/** Wizard form. It is saved as a draft: it must never hold a secret. */
export interface UserCreateForm {
  name: string;
  permission: string;
  grantMode: GrantMode;
  selectedGroupIds: string[];
  selectedGroupBuckets: string[];
  customExtraBuckets: string[];
  buckets: string;
  createKey: boolean;
  description: string;
}

function initialForm(): UserCreateForm {
  return {
    name: '',
    permission: 'read-write',
    grantMode: 'all',
    selectedGroupIds: [],
    selectedGroupBuckets: [],
    customExtraBuckets: [],
    buckets: '*',
    createKey: true,
    description: '',
  };
}

export function useUserCreate() {
  const users = useTuttiUsersStore();
  const protection = useTuttiProtectionStore();
  const form = useWizardDraft<UserCreateForm>(
    USER_CREATE_DRAFT_KEY,
    initialForm,
  );
  const busy = ref(false);
  /** i18n key of the last submit problem (or API message), '' when none */
  const error = ref('');
  const clusterBuckets = ref<string[]>([]);
  const discovered = ref<StorageBucketEntry[]>([]);
  /** shown once, kept in memory only: never in the draft, storage or URL */
  const created = ref<TuttiProxyAccountSecret | null>(null);
  // Backend credentials must stay in memory, outside the persisted wizard draft.
  const credentials = ref<S3UserCredentialInput[]>([]);
  const credentialsLoaded = ref(false);
  const credentialsLoading = ref(false);
  const credentialsError = ref(false);
  const verifyCredentials = ref(true);
  const credentialsValid = computed(
    () =>
      credentialsLoaded.value &&
      credentials.value.every(
        (row) =>
          row.accessKeyId.trim() !== '' && row.secretAccessKey.trim() !== '',
      ),
  );

  async function loadCredentialRequirements() {
    credentialsLoading.value = true;
    credentialsError.value = false;
    credentialsLoaded.value = false;

    try {
      const { storages } = await RuntimeStorageService.list();

      credentials.value = (storages ?? [])
        .filter((storage) => !storage.connectionAccount?.configured)
        .map(
          (storage) =>
            credentials.value.find((row) => row.storage === storage.name) ?? {
              storage: storage.name,
              accessKeyId: '',
              secretAccessKey: '',
            },
        );
      credentialsLoaded.value = true;
    } catch {
      credentialsError.value = true;
    } finally {
      credentialsLoading.value = false;
    }
  }

  const nameProblem = computed(() =>
    form.value.name === ''
      ? null
      : UserHelper.validateName(
          form.value.name,
          users.users.map((u) => u.name),
        ),
  );

  const storageGroups = computed(() =>
    storageScopeGroups(protection.groups, form.value.selectedGroupIds),
  );

  const availableGroupBuckets = computed<GroupBucketItem[]>(() =>
    groupBucketItems(
      protection.groups,
      form.value.selectedGroupIds,
      discovered.value,
    ),
  );

  /** Buckets the policy will list, or ['*'] when it is unrestricted. */
  const effectiveBuckets = computed<string[]>(() => {
    const f = form.value;

    if (f.grantMode === 'all') return ['*'];

    if (f.grantMode === 'groups') {
      const combined = [...f.selectedGroupBuckets, ...f.customExtraBuckets];

      return combined.length > 0 ? combined : ['*'];
    }

    const parsed = UserHelper.parseBucketPatterns(f.buckets);

    return parsed.patterns.length > 0 ? parsed.patterns : ['*'];
  });

  /** i18n key of what blocks the access section, null when it is fine */
  const permissionProblem = computed<string | null>(() => {
    const f = form.value;

    if (f.grantMode === 'groups') {
      if (f.selectedGroupIds.length === 0) return 'noGroupSelectedHint';

      if (
        storageGroups.value.length === 0 &&
        f.selectedGroupBuckets.length === 0 &&
        f.customExtraBuckets.length === 0
      )
        return 'bucketsRequired';
    } else if (f.grantMode === 'custom') {
      return UserHelper.validatePolicy(f.buckets, true);
    }

    return null;
  });

  const identityValid = computed(
    () =>
      form.value.name !== '' &&
      nameProblem.value === null &&
      credentialsValid.value,
  );
  const permissionValid = computed(() => permissionProblem.value === null);
  const formValid = computed(
    () => identityValid.value && permissionValid.value,
  );

  async function discoverStorageBuckets() {
    const storages = storagesToQuery(storageGroups.value);

    if (storages.length === 0) {
      discovered.value = [];

      return;
    }

    try {
      discovered.value =
        await BucketDiscoveryHelper.discoverStorageBuckets(storages);
    } catch {
      discovered.value = [];
    }
  }

  async function loadClusterBuckets() {
    try {
      clusterBuckets.value = await BucketDiscoveryHelper.discoverBuckets();
    } catch {
      clusterBuckets.value = [];
    }
  }

  async function setGroups(ids: string[]) {
    const previous = form.value.selectedGroupIds;

    form.value.selectedGroupIds = ids;
    await discoverStorageBuckets();
    form.value.selectedGroupBuckets = nextGroupBuckets(
      protection.groups,
      previous,
      ids,
      form.value.selectedGroupBuckets,
      discovered.value,
    );
  }

  function setGroupBucket(bucket: string, checked: boolean) {
    const rest = form.value.selectedGroupBuckets.filter((b) => b !== bucket);

    form.value.selectedGroupBuckets = checked ? [...rest, bucket] : rest;
  }

  function selectAllGroupBuckets(all: boolean) {
    form.value.selectedGroupBuckets = all
      ? availableGroupBuckets.value.map((b) => b.bucket)
      : [];
  }

  /** @returns an i18n key when the pattern is refused */
  function addExtraPattern(raw: string): string | null {
    const pattern = raw.trim();

    if (!pattern) return null;

    if (AccessPolicyHelper.validatePattern(pattern)) return 'invalidBuckets';

    if (!form.value.customExtraBuckets.includes(pattern))
      form.value.customExtraBuckets.push(pattern);

    return null;
  }

  function removeExtraPattern(pattern: string) {
    form.value.customExtraBuckets = form.value.customExtraBuckets.filter(
      (p) => p !== pattern,
    );
  }

  function addSuggestedBucket(bucket: string) {
    const current = UserHelper.parseBucketPatterns(form.value.buckets).patterns;
    const rest = current.filter((p) => p !== '*');

    if (!rest.includes(bucket))
      form.value.buckets = [...rest, bucket].join(', ');
  }

  async function init() {
    await Promise.all([
      loadClusterBuckets(),
      users.load(),
      loadCredentialRequirements(),
      protection.refresh().catch(() => null),
    ]);
  }

  let submitted = false;

  /** Creates the user once; the secret stays in `created` (memory only). */
  async function submit(): Promise<{ name: string; secret: string }> {
    if (busy.value || !identityValid.value || !permissionValid.value) {
      throw new Error('completeIdentityAndCredentials');
    }

    const name = form.value.name;

    busy.value = true;
    error.value = '';

    try {
      const result = await TuttiService.createUser({
        name: form.value.name,
        credentials: credentials.value.map((row) => ({
          ...row,
          accessKeyId: row.accessKeyId.trim(),
        })),
        verify: credentials.value.length > 0 && verifyCredentials.value,
        createAccessKey: form.value.createKey,
        description: form.value.description,
        policy: {
          user: form.value.name,
          permission: form.value.permission,
          buckets: effectiveBuckets.value,
        },
      });

      created.value = result.accessKey ?? null;
      credentials.value = [];
      // the draft stays until dismissSecret(): clearing it now would reset the
      // form and reset the form behind the one-time secret modal
      submitted = true;

      return { name, secret: result.accessKey?.secretAccessKey ?? '' };
    } finally {
      busy.value = false;
    }
  }

  function dismissSecret() {
    created.value = null;
    credentials.value = [];

    if (submitted) {
      submitted = false;
      clearWizardDraft(USER_CREATE_DRAFT_KEY);
    }
  }

  return {
    form,
    busy,
    error,
    created,
    credentials,
    credentialsValid,
    credentialsLoaded,
    credentialsLoading,
    credentialsError,
    verifyCredentials,
    loadCredentialRequirements,
    formValid,
    identityValid,
    permissionValid,
    permissionProblem,
    nameProblem,
    clusterBuckets,
    storageGroups,
    availableGroupBuckets,
    effectiveBuckets,
    init,
    submit,
    dismissSecret,
    setGroups,
    setGroupBucket,
    selectAllGroupBuckets,
    addExtraPattern,
    removeExtraPattern,
    addSuggestedBucket,
  };
}

export type UserCreateController = ReturnType<typeof useUserCreate>;

/** The page provides the controller; the form sections inject it. */
export const USER_CREATE_KEY: InjectionKey<UserCreateController> =
  Symbol('userCreate');

export function injectUserCreate(): UserCreateController {
  const controller = inject(USER_CREATE_KEY);

  if (!controller) throw new Error('useUserCreate is not provided');

  return controller;
}
