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
  /**
   * Step 1: where to copy from and to. Two labelled selects; the destination
   * list leaves out the source, and picking a source that equals the
   * destination clears the destination.
   */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NFormItem, NSelect } from 'naive-ui';
  import i18nAddReplication from './i18nAddReplication';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';

  const { t } = useI18n({ messages: i18nAddReplication });
  const { storages, selectedFromStorage, selectedToStorage } = storeToRefs(
    useTuttiAddReplicationStore(),
  );

  const ordered = computed(() =>
    GeneralHelper.orderBy(storages.value, 'name', 'asc'),
  );
  const sourceOptions = computed(() =>
    ordered.value.map((storage) => ({
      label: storage.name,
      value: storage.name,
    })),
  );
  const destinationOptions = computed(() =>
    ordered.value.map((storage) => ({
      label: storage.name,
      value: storage.name,
      disabled: storage.name === selectedFromStorage.value?.name,
    })),
  );

  const find = (name: string | null) =>
    storages.value.find((storage) => storage.name === name) ?? null;

  const source = computed({
    get: () => selectedFromStorage.value?.name ?? null,
    set: (name: string | null) => {
      selectedFromStorage.value = find(name);

      if (selectedToStorage.value?.name === name)
        selectedToStorage.value = null;
    },
  });
  const destination = computed({
    get: () => selectedToStorage.value?.name ?? null,
    set: (name: string | null) => {
      selectedToStorage.value = find(name);
    },
  });
</script>

<template>
  <div class="storages-step">
    <NFormItem :label="t('sourceLabel')">
      <NSelect
        v-model:value="source"
        :options="sourceOptions"
        filterable
      />
    </NFormItem>
    <NFormItem :label="t('destinationLabel')">
      <NSelect
        v-model:value="destination"
        :options="destinationOptions"
        filterable
      />
    </NFormItem>
  </div>
</template>

<style scoped lang="scss">
  .storages-step {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    column-gap: var(--tt-space-5);
    max-width: 720px;

    @media screen and (max-width: 639px) {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>
