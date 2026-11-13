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

import { defineComponent, h, type PropType } from 'vue';
import StateBlock from './StateBlock.vue';

const common = {
  title: { type: String, default: undefined },
  description: { type: String, default: undefined },
};

/** Skeleton placeholder (appears after 100ms, so fast loads never flash). */
export const LoadingBlock = defineComponent({
  name: 'LoadingBlock',
  props: {
    title: common.title,
    rows: { type: Number, default: 4 },
  },
  setup(props) {
    return () => h(StateBlock, { kind: 'loading', ...props });
  },
});

/** Empty state; put the call to action (a verb button) in the default slot. */
export const EmptyBlock = defineComponent({
  name: 'EmptyBlock',
  props: common,
  setup(props, { slots }) {
    return () => h(StateBlock, { kind: 'empty', ...props }, slots);
  },
});

/** Error state with a "Try again" button that emits `retry`. */
export const ErrorBlock = defineComponent({
  name: 'ErrorBlock',
  props: {
    ...common,
    retryLabel: { type: String as PropType<string>, default: undefined },
  },
  emits: ['retry'],
  setup(props, { emit }) {
    return () =>
      h(StateBlock, {
        kind: 'error',
        ...props,
        onRetry: () => emit('retry'),
      });
  },
});
