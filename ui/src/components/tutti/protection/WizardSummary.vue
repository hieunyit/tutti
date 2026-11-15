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
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nProtection from './i18nProtection';
  import { EMPTY_VALUE } from '@/components/kit';
  import type { ProtectionGroupForm } from '@/utils/types/protection';

  const props = defineProps<{ form: ProtectionGroupForm }>();
  const { t } = useI18n({ messages: i18nProtection });

  const buckets = computed(() =>
    props.form.buckets.length
      ? props.form.buckets.join(', ')
      : t('wizAllBuckets'),
  );
  const storages = computed(() =>
    props.form.members.length ? props.form.members.join(' → ') : EMPTY_VALUE,
  );
</script>

<template>
  <dl class="pg-summary">
    <div>
      <dt>{{ t('fieldId') }}</dt>
      <dd class="tt-mono">{{ form.id.trim() || EMPTY_VALUE }}</dd>
    </div>
    <div>
      <dt>{{ t('colScope') }}</dt>
      <dd>{{ buckets }}</dd>
    </div>
    <div>
      <dt>{{ t('colStorages') }}</dt>
      <dd class="tt-mono">{{ storages }}</dd>
    </div>
    <div>
      <dt>{{ t('fieldCopies') }}</dt>
      <dd>{{ form.copies }}</dd>
    </div>
    <div>
      <dt>{{ t('fieldMode') }}</dt>
      <dd>{{ form.mode === 'auto' ? t('modeAuto') : t('modeManual') }}</dd>
    </div>
  </dl>
</template>

<style lang="scss" scoped>
  .pg-summary {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);
    margin: 0;

    dt {
      color: var(--tt-text-3);
      font-size: var(--tt-text-xs);
    }

    dd {
      margin: var(--tt-space-1) 0 0;
      font-weight: var(--tt-weight-medium);
      overflow-wrap: anywhere;
    }
  }
</style>
