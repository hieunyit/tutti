/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

/** State and save logic of the access policy drawer. */
import { storeToRefs } from 'pinia';
import { computed, ref, watch, type Ref } from 'vue';
import {
  availableGroupBuckets,
  bucketsFromGroups,
  grantStateFor,
  matchingGroups,
  nextGroupBuckets,
  storageScopeGroups,
  storagesToQuery,
  type GrantMode,
} from './policyBuckets';
import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import { AccessPolicyHelper } from '@/utils/helpers/AccessPolicyHelper';
import {
  BucketDiscoveryHelper,
  type StorageBucketEntry,
} from '@/utils/helpers/BucketDiscoveryHelper';
import type {
  TuttiAccessPolicyForm,
  TuttiAccessPolicyValidationError,
  TuttiProxyAccount,
} from '@/utils/types/tutti';

export function useAccessPolicyForm(
  show: Ref<boolean>,
  account: Ref<TuttiProxyAccount | null>,
  close: () => void,
) {
  const store = useTuttiAccessPolicyStore();
  const protection = useTuttiProtectionStore();
  const { isSaving, saveError } = storeToRefs(store);

  const form = ref<TuttiAccessPolicyForm>(AccessPolicyHelper.policyToForm(''));
  const grantMode = ref<GrantMode>('all');
  const selectedGroupIds = ref<string[]>([]);
  const selectedGroupBuckets = ref<string[]>([]);
  const extraBuckets = ref<string[]>([]);
  const clusterBuckets = ref<string[]>([]);
  const discovered = ref<StorageBucketEntry[]>([]);
  const extraInput = ref('');
  const customInput = ref('');
  const showErrors = ref(false);

  const scopeGroups = computed(() =>
    storageScopeGroups(protection.groups, selectedGroupIds.value),
  );
  const groupBuckets = computed(() =>
    availableGroupBuckets(
      protection.groups,
      selectedGroupIds.value,
      discovered.value,
    ),
  );

  async function discoverFor(groups = scopeGroups.value) {
    const names = storagesToQuery(groups);

    if (names.length === 0) {
      discovered.value = [];

      return;
    }

    try {
      discovered.value = await BucketDiscoveryHelper.discoverStorageBuckets(
        names,
        account.value?.user,
      );
    } catch {
      discovered.value = [];
    }
  }

  async function loadClusterBuckets() {
    try {
      clusterBuckets.value = await BucketDiscoveryHelper.discoverBuckets(
        account.value?.user,
      );
    } catch {
      clusterBuckets.value = [];
    }
  }

  function syncBuckets() {
    form.value.buckets = bucketsFromGroups(
      scopeGroups.value,
      selectedGroupBuckets.value,
      extraBuckets.value,
    );
  }

  function setGrantMode(mode: GrantMode) {
    grantMode.value = mode;

    if (mode === 'all') form.value.buckets = ['*'];
    else if (mode === 'groups') syncBuckets();
    else if (form.value.buckets.length === 1 && form.value.buckets[0] === '*') {
      form.value.buckets = [];
    }
  }

  async function setGroups(ids: string[]) {
    const previous = selectedGroupIds.value;

    selectedGroupIds.value = ids;
    await discoverFor();
    selectedGroupBuckets.value = nextGroupBuckets(
      protection.groups,
      previous,
      ids,
      selectedGroupBuckets.value,
      discovered.value,
    );
    syncBuckets();
  }

  function toggleGroupBucket(bucket: string, checked: boolean) {
    selectedGroupBuckets.value = checked
      ? [...new Set([...selectedGroupBuckets.value, bucket])]
      : selectedGroupBuckets.value.filter((b) => b !== bucket);
    syncBuckets();
  }

  function setAllGroupBuckets(all: boolean) {
    selectedGroupBuckets.value = all
      ? groupBuckets.value.map((b) => b.bucket)
      : [];
    syncBuckets();
  }

  function addExtra() {
    const pattern = extraInput.value.trim();

    if (!pattern || AccessPolicyHelper.validatePattern(pattern)) return;

    if (!extraBuckets.value.includes(pattern)) extraBuckets.value.push(pattern);

    extraInput.value = '';
    syncBuckets();
  }

  function removeExtra(pattern: string) {
    extraBuckets.value = extraBuckets.value.filter((p) => p !== pattern);
    syncBuckets();
  }

  function addSuggested(bucket: string) {
    if (!form.value.buckets.includes(bucket)) form.value.buckets.push(bucket);
  }

  /** the not-yet-added input counts too so a typo is reported before saving */
  const patternError = computed<TuttiAccessPolicyValidationError | null>(() =>
    customInput.value === ''
      ? null
      : AccessPolicyHelper.validatePattern(customInput.value),
  );

  function addCustom() {
    if (customInput.value === '' || patternError.value) return;

    form.value.buckets = AccessPolicyHelper.addPattern(
      form.value.buckets,
      customInput.value,
    );
    customInput.value = '';
  }

  function removeCustom(pattern: string) {
    form.value.buckets = AccessPolicyHelper.removePattern(
      form.value.buckets,
      pattern,
    );
  }

  const errors = computed(() => AccessPolicyHelper.validate(form.value));
  const patternErrors = computed<TuttiAccessPolicyValidationError[]>(() => {
    const codes = errors.value.filter((e) => e.startsWith('PATTERN_'));
    const all = patternError.value ? [...codes, patternError.value] : codes;

    return showErrors.value || patternError.value ? [...new Set(all)] : [];
  });

  async function open() {
    if (!account.value) return;

    if (!protection.isLoaded && !protection.isLoading) protection.refresh();

    loadClusterBuckets();
    form.value = AccessPolicyHelper.policyToForm(
      account.value.user,
      account.value.policy,
    );
    customInput.value = '';
    extraInput.value = '';
    showErrors.value = false;
    store.saveError = null;

    const matching = matchingGroups(form.value.buckets, protection.groups);

    selectedGroupIds.value = matching.map((g) => g.id);
    await discoverFor(storageScopeGroups(matching, selectedGroupIds.value));

    const state = grantStateFor(form.value.buckets, matching, discovered.value);

    grantMode.value = state.mode;
    selectedGroupIds.value = state.groupIds;
    selectedGroupBuckets.value = state.groupBuckets;
    extraBuckets.value = state.extraBuckets;
  }

  async function submit() {
    showErrors.value = true;

    // a pending, valid pattern is added implicitly
    if (grantMode.value === 'custom' && customInput.value !== '') {
      if (patternError.value) return;

      addCustom();
    }

    if (grantMode.value === 'groups' && extraInput.value.trim() !== '')
      addExtra();

    if (AccessPolicyHelper.validate(form.value).length > 0) return;

    if (await store.savePolicy(form.value)) close();
  }

  watch(show, (value) => {
    if (value) open();
  });

  return {
    form,
    grantMode,
    selectedGroupIds,
    selectedGroupBuckets,
    extraBuckets,
    clusterBuckets,
    extraInput,
    customInput,
    scopeGroups,
    groupBuckets,
    patternError,
    patternErrors,
    isSaving,
    saveError,
    setGrantMode,
    setGroups,
    toggleGroupBucket,
    setAllGroupBuckets,
    addExtra,
    removeExtra,
    addSuggested,
    addCustom,
    removeCustom,
    submit,
  };
}
