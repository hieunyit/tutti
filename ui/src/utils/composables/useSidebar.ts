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

import { onBeforeUnmount, ref } from 'vue';
import LocalStorageHelper from '@/utils/helpers/LocalStorageHelper';
import { LocalStorageItem } from '@/utils/types/localStorage';
import { SIDEBAR_MOBILE_BREAKPOINT } from '@/utils/constants/navigation';

function readCollapsed(): boolean {
  try {
    return (
      LocalStorageHelper.get<string>(LocalStorageItem.SIDEBAR_COLLAPSED) ===
      'true'
    );
  } catch {
    return false;
  }
}

function writeCollapsed(value: boolean) {
  try {
    LocalStorageHelper.set(LocalStorageItem.SIDEBAR_COLLAPSED, String(value));
  } catch {
    // storage unavailable: the preference just is not persisted
  }
}

/** Sidebar state: persisted collapsed flag, mobile drawer flag and viewport detection. */
export function useSidebar() {
  const isCollapsed = ref(readCollapsed());
  const isMobile = ref(false);
  const isDrawerOpen = ref(false);
  let mediaQuery: MediaQueryList | null = null;

  function onMediaChange(event: MediaQueryListEvent | MediaQueryList) {
    isMobile.value = event.matches;

    if (!event.matches) {
      isDrawerOpen.value = false;
    }
  }

  function toggleCollapsed() {
    isCollapsed.value = !isCollapsed.value;
    writeCollapsed(isCollapsed.value);
  }

  // evaluated in setup (not onMounted) to avoid a desktop-layout flash on phones
  if (typeof window !== 'undefined' && window.matchMedia) {
    mediaQuery = window.matchMedia(
      `(max-width: ${SIDEBAR_MOBILE_BREAKPOINT - 1}px)`,
    );
    onMediaChange(mediaQuery);
    mediaQuery.addEventListener('change', onMediaChange);
  }

  onBeforeUnmount(() => {
    mediaQuery?.removeEventListener('change', onMediaChange);
  });

  return { isCollapsed, isMobile, isDrawerOpen, toggleCollapsed };
}
