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
  import { NModal } from 'naive-ui';
  import { CButton } from '@clyso/clyso-ui-kit';
  import { useI18n } from 'vue-i18n';
  import i18nLayout from './i18nLayout';
  import { useAboutDialog } from '@/utils/composables/useAboutDialog';
  import { APP_VERSION } from '@/utils/constants/env';

  const CHORUS_URL = 'https://github.com/clyso/chorus';
  const TUTTI_URL = 'https://github.com/hieunyit/tutti';

  const { t } = useI18n({ messages: i18nLayout });
  const { isAboutOpen } = useAboutDialog();
</script>

<template>
  <NModal
    v-model:show="isAboutOpen"
    preset="card"
    class="about-dialog"
    :title="t('aboutTitle')"
    :bordered="false"
    style="width: min(480px, calc(100vw - 32px))"
  >
    <p class="about-dialog__name">
      Tutti
      <span class="about-dialog__version"
        >{{ t('version') }} {{ APP_VERSION }}</span
      >
    </p>
    <p class="about-dialog__text">{{ t('productTagline') }}</p>

    <p class="about-dialog__text">
      {{ t('attributionBefore') }}
      <a
        :href="CHORUS_URL"
        rel="noopener"
        target="_blank"
        >Chorus</a
      >
      {{ t('attributionAfter') }}
    </p>

    <p class="about-dialog__text">
      <a
        :href="TUTTI_URL"
        rel="noopener"
        target="_blank"
        >{{ t('sourceCode') }}</a
      >
    </p>

    <template #footer>
      <div class="about-dialog__footer">
        <CButton @click="isAboutOpen = false">{{ t('close') }}</CButton>
      </div>
    </template>
  </NModal>
</template>

<style lang="scss" scoped>
  .about-dialog {
    &__name {
      margin: 0 0 4px;
      font-size: 20px;
      font-weight: 600;
    }

    &__version {
      margin-left: 8px;
      font-size: 14px;
      font-weight: 400;
      color: var(--text-color-3);
    }

    &__text {
      margin: 0 0 12px;
      line-height: 22px;
      color: var(--text-color-2);

      a {
        color: var(--primary-color);
      }
    }

    &__footer {
      display: flex;
      justify-content: flex-end;
    }
  }
</style>
