<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->
<script setup lang="ts">
  /** Filter controls of the routing rules toolbar (shown in the Filters panel). */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NFormItem, NSelect } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { ALL_BUCKETS } from './routingColumns';
  import i18nRouting from '@/pages/TuttiRoutingPage/i18nRouting';
  import { useTuttiRoutingPoliciesStore } from '@/stores/tuttiRoutingPoliciesStore';
  import { RoutingPolicyStatusFilter } from '@/utils/types/tuttiCore';

  const { t } = useI18n({ messages: i18nRouting });
  const store = useTuttiRoutingPoliciesStore();
  const {
    routingPolicies,
    filterUsers,
    filterBuckets,
    filterStorages,
    filterStatus,
  } = storeToRefs(store);

  const options = (values: string[]) =>
    [...new Set(values.filter(Boolean))]
      .sort()
      .map((value) => ({ label: value, value }));

  const userOptions = computed(() =>
    options(routingPolicies.value.map((row) => row.user)),
  );
  const bucketOptions = computed(() =>
    options(
      routingPolicies.value
        .map((row) => row.bucket)
        .filter((bucket) => bucket !== ALL_BUCKETS),
    ),
  );
  const storageOptions = computed(() =>
    options(routingPolicies.value.map((row) => row.toStorage)),
  );
  const statusOptions = computed(() => [
    { label: t('statusAllowed'), value: RoutingPolicyStatusFilter.ALLOWED },
    { label: t('statusBlocked'), value: RoutingPolicyStatusFilter.BLOCKED },
  ]);
</script>

<template>
  <NFormItem
    :label="t('colUser')"
    label-placement="left"
    :show-feedback="false"
  >
    <NSelect
      v-model:value="filterUsers"
      :options="userOptions"
      multiple
      filterable
      clearable
      size="small"
    />
  </NFormItem>
  <NFormItem
    :label="t('colBucket')"
    label-placement="left"
    :show-feedback="false"
  >
    <NSelect
      v-model:value="filterBuckets"
      :options="bucketOptions"
      multiple
      filterable
      clearable
      size="small"
    />
  </NFormItem>
  <NFormItem
    :label="t('colStorage')"
    label-placement="left"
    :show-feedback="false"
  >
    <NSelect
      v-model:value="filterStorages"
      :options="storageOptions"
      multiple
      filterable
      clearable
      size="small"
    />
  </NFormItem>
  <NFormItem
    :label="t('colStatus')"
    label-placement="left"
    :show-feedback="false"
  >
    <NSelect
      v-model:value="filterStatus"
      :options="statusOptions"
      clearable
      size="small"
    />
  </NFormItem>
</template>
