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
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import type { TuttiReplication } from '@/utils/types/tuttiCore';
  import type { AddId } from '@/utils/types/helper';
  import TuttiDirectionCell from '@/components/tutti/common/TuttiDirectionCell/TuttiDirectionCell.vue';

  const { t } = useI18n({
    messages: i18nReplications,
  });

  defineProps<{
    replication: AddId<TuttiReplication>;
  }>();
</script>

<template>
  <TuttiDirectionCell
    :from-text="replication.id.fromStorage"
    :to-text="replication.id.toStorage"
    :from-tooltip="t('replicationFrom')"
    :to-tooltip="t('replicationTo')"
    to-type="warning"
  />
</template>
