/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

// Shared UI kit (spec section 3). Pages import from '@/components/kit' only;
// do not write your own table, confirm dialog or loading/empty/error state.

export { default as PageShell } from './PageShell.vue';
export { default as DataTable } from './DataTable.vue';
export { default as FilterBar } from './FilterBar.vue';
export { default as StatTile } from './StatTile.vue';
export { default as DetailLayout } from './DetailLayout.vue';
export { default as FormSection } from './FormSection.vue';
export { default as WizardShell } from './WizardShell.vue';
export { default as ConfirmDialog } from './ConfirmDialog.vue';
export { default as StateBlock } from './StateBlock.vue';
export { default as StatusBadge } from './StatusBadge.vue';
export { default as CopyValue } from './CopyValue.vue';
export { LoadingBlock, EmptyBlock, ErrorBlock } from './stateBlocks';

export {
  useQueryTab,
  useTableDensity,
  useWizardDraft,
  clearWizardDraft,
  type TableDensity,
} from './composables';

export type {
  StatusTone,
  PageState,
  PageShellTab,
  WizardStep,
  FilterChip,
  StatTrend,
} from './types';

// Older building blocks without a kit replacement, re-exported so new code
// has one import path. Their files stay where they are (old imports work).
// Only modules free of @clyso/clyso-ui-kit, so the kit mounts in unit tests.
export { default as BackLink } from '@/components/tutti/common/BackLink.vue';
export { default as SearchInput } from '@/components/tutti/common/SearchInput.vue';
export { default as SegmentedFilter } from '@/components/tutti/common/SegmentedFilter.vue';
export { default as TagChip } from '@/components/tutti/common/TagChip.vue';
export { default as Callout } from '@/components/ui/Callout.vue';
export { default as SectionCard } from '@/components/ui/SectionCard.vue';
export {
  mapStatusToVariant,
  orDash,
  EMPTY_VALUE,
} from '@/components/ui/status';
