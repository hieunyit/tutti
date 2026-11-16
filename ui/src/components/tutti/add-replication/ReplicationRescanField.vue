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
   * Optional "pick up files written directly to the source": how often the
   * source is rescanned. Off by default; one helper sentence says when to use it.
   */
  import { computed, useId } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NSelect } from 'naive-ui';
  import i18nReplicationRescan from '@/components/tutti/replications/i18nReplicationRescan';
  import {
    RESCAN_MINUTES,
    RESCAN_SUGGESTED_MINUTES,
  } from '@/utils/helpers/ReplicationRescanHelper';

  const { t } = useI18n({ messages: i18nReplicationRescan });
  const model = defineModel<number>({ required: true });
  const uid = useId();

  const options = computed(() =>
    RESCAN_MINUTES.map((minutes) => {
      const label = minutes === 0 ? t('rescanOff') : t(`rescanOpt${minutes}`);

      return {
        label:
          minutes === RESCAN_SUGGESTED_MINUTES
            ? `${label} · ${t('rescanRecommended')}`
            : label,
        value: minutes,
      };
    }),
  );
</script>

<template>
  <fieldset class="rescan">
    <legend class="rescan__legend">{{ t('rescanLabel') }}</legend>
    <label :for="`rescan-select-${uid}`">{{ t('rescanSelect') }}</label>
    <NSelect
      :value="model"
      :options="options"
      :input-props="{
        id: `rescan-select-${uid}`,
        'aria-describedby': `rescan-help-${uid}`,
      }"
      @update:value="(value: number) => (model = value)"
    />
    <p
      :id="`rescan-help-${uid}`"
      class="rescan__help"
    >
      {{ t('rescanHelp') }}
    </p>
  </fieldset>
</template>

<style lang="scss" scoped>
  .rescan {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);
    min-width: 0;
    max-width: 420px;
    margin: 0;
    padding: 0;
    border: 0;

    &__legend {
      padding: 0;
      margin-bottom: var(--tt-space-2);
      font-weight: 600;
    }

    &__help {
      margin: 0;
      font-size: 0.875rem;
      opacity: 0.8;
    }
  }
</style>
