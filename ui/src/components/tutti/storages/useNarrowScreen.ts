/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { onBeforeUnmount, onMounted, ref } from 'vue';

/** True below `maxWidth` px (tables become cards); false without matchMedia. */
export function useNarrowScreen(maxWidth = 640) {
  const narrow = ref(false);
  let query: MediaQueryList | null = null;
  const sync = () => {
    narrow.value = query?.matches ?? false;
  };

  onMounted(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return;

    query = window.matchMedia(`(max-width: ${maxWidth - 1}px)`);
    sync();
    query.addEventListener('change', sync);
  });
  onBeforeUnmount(() => query?.removeEventListener('change', sync));

  return narrow;
}
