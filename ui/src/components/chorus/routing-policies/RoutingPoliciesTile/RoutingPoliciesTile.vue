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
  import { useI18n } from 'vue-i18n';
  import RoutingPoliciesList from '../RoutingPoliciesList/RoutingPoliciesList.vue';
  import RoutingPoliciesFilters from '../RoutingPoliciesFilters/RoutingPoliciesFilters.vue';
  import RoutingPoliciesListActions from '../RoutingPoliciesListActions/RoutingPoliciesListActions.vue';
  import i18nRoutingPolicies from '@/components/chorus/routing-policies/i18nRoutingPolicies';
  import ListCard from '@/components/tutti/common/ListCard.vue';
  import RoutingPoliciesListTags from '@/components/chorus/routing-policies/RoutingPoliciesListTags/RoutingPoliciesListTags.vue';

  const { t } = useI18n({ messages: i18nRoutingPolicies });
</script>

<template>
  <ListCard :title="t('routingListTitle')">
    <template #filters>
      <RoutingPoliciesFilters />
    </template>
    <template #tags>
      <RoutingPoliciesListTags />
    </template>
    <template #actions>
      <RoutingPoliciesListActions />
    </template>

    <RoutingPoliciesList />
  </ListCard>
</template>
