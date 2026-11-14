<!--
  - Copyright © 2026 Clyso GmbH
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
  import { CIcon, CTag, CTooltip } from '@clyso/clyso-ui-kit';

  type CTagProps = InstanceType<typeof CTag>['$props'];

  const props = withDefaults(
    defineProps<{
      storageName: string;
      tooltip?: string;
      size?: CTagProps['size'];
      type?: CTagProps['type'];
      iconName?: string;
    }>(),
    {
      size: 'small',
      type: 'success',
      tooltip: undefined,
      icon: undefined,
    },
  );
</script>

<template>
  <div class="tutti-storage-tag">
    <CTooltip :delay="500">
      <template #trigger>
        <CTag
          :bordered="false"
          round
          class="tutti-storage-tag__storage-name"
          :size="props.size"
          :type="props.type"
        >
          <template
            #icon
            v-if="props.iconName"
          >
            <CIcon
              :is-inline="true"
              :name="props.iconName"
            />
          </template>
          <span class="tutti-storage-tag__text">{{ props.storageName }}</span>
        </CTag>
      </template>
      <span class="tutti-storage-tag__storage-tooltip">
        <strong>{{ props.storageName }}</strong>
        <template v-if="props.tooltip">: {{ props.tooltip }}</template>
      </span>
    </CTooltip>
  </div>
</template>

<style lang="scss" scoped>
  .tutti-storage-tag {
    display: inline-flex;
    min-width: 0;
    max-width: 100%;

    &__storage-name {
      max-width: 150px;
    }

    &__text {
      display: inline-block;
      max-width: 130px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      vertical-align: middle;
    }
  }
</style>
