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
  import {
    CDashboardFooter,
    type DashboardFooterLink,
    I18nLocale,
  } from '@clyso/clyso-ui-kit';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';

  const CHORUS_URL = 'https://github.com/clyso/chorus';
  const TUTTI_URL = 'https://github.com/hieunyit/tutti';

  enum DashboardFooterLinkId {
    TUTTI_GITHUB = 'TUTTI_GITHUB',
    BASED_ON_PREFIX = 'BASED_ON_PREFIX',
    BASED_ON_SUFFIX = 'BASED_ON_SUFFIX',
  }

  const { t } = useI18n({
    messages: {
      [I18nLocale.EN]: {
        [DashboardFooterLinkId.TUTTI_GITHUB]: 'Tutti on GitHub',
        [DashboardFooterLinkId.BASED_ON_PREFIX]: 'Tutti · based on',
        [DashboardFooterLinkId.BASED_ON_SUFFIX]:
          'by Clyso GmbH (https://github.com/clyso/chorus), licensed under AGPL-3.0 (UI) and Apache-2.0 (backend)',
      },
      [I18nLocale.DE]: {
        [DashboardFooterLinkId.TUTTI_GITHUB]: 'Tutti auf GitHub',
        [DashboardFooterLinkId.BASED_ON_PREFIX]: 'Tutti · basiert auf',
        [DashboardFooterLinkId.BASED_ON_SUFFIX]:
          'von Clyso GmbH (https://github.com/clyso/chorus), lizenziert unter AGPL-3.0 (UI) und Apache-2.0 (Backend)',
      },
    },
  });

  const footerLinks = computed<DashboardFooterLink[]>(() => [
    {
      id: DashboardFooterLinkId.TUTTI_GITHUB,
      text: t(DashboardFooterLinkId.TUTTI_GITHUB),
      url: TUTTI_URL,
    },
  ]);
</script>

<template>
  <CDashboardFooter
    class="dashboard-footer"
    :is-centered="true"
    :links="footerLinks"
  >
    <template #copyright>
      {{ t(DashboardFooterLinkId.BASED_ON_PREFIX) }}
      <a
        class="dashboard-footer__chorus-link"
        :href="CHORUS_URL"
        rel="noopener"
        target="_blank"
        >Chorus</a
      >
      {{ t(DashboardFooterLinkId.BASED_ON_SUFFIX) }}
    </template>
  </CDashboardFooter>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .dashboard-footer {
    ::v-deep(.container) {
      @include utils.desktop {
        padding: 0 utils.unit(8);
        max-width: none;
      }
    }

    &__chorus-link {
      color: inherit;
      text-decoration: underline;
      text-underline-offset: 2px;

      &:hover {
        opacity: 0.8;
      }
    }
  }
</style>
