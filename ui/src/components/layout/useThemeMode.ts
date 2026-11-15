/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { storeToRefs } from 'pinia';
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ColorScheme } from '@clyso/clyso-ui-kit';
import { useColorSchemeStore } from '@/stores/colorSchemeStore';
import LocalStorageHelper from '@/utils/helpers/LocalStorageHelper';
import { LocalStorageItem } from '@/utils/types/localStorage';

export type ThemeMode = 'light' | 'dark' | 'system';

/**
 * Light / dark / system theme state. "System" is the absence of a saved
 * choice: the store then follows prefers-color-scheme (also live).
 */
export function useThemeMode() {
  const store = useColorSchemeStore();
  const { isDark } = storeToRefs(store);
  const media =
    typeof window.matchMedia === 'function'
      ? window.matchMedia('(prefers-color-scheme: dark)')
      : null;

  const mode = ref<ThemeMode>(
    LocalStorageHelper.get(LocalStorageItem.COLOR_SCHEME) === null
      ? 'system'
      : isDark.value
        ? 'dark'
        : 'light',
  );

  function followSystem() {
    store.setColorScheme(media?.matches ? ColorScheme.DARK : ColorScheme.LIGHT);
    LocalStorageHelper.remove(LocalStorageItem.COLOR_SCHEME);
  }

  function select(key: string | number) {
    mode.value = key as ThemeMode;

    if (key === 'system') {
      followSystem();

      return;
    }

    store.setColorScheme(key === 'dark' ? ColorScheme.DARK : ColorScheme.LIGHT);
  }

  function onSystemChange() {
    if (mode.value === 'system') followSystem();
  }

  onMounted(() => media?.addEventListener('change', onSystemChange));
  onBeforeUnmount(() => media?.removeEventListener('change', onSystemChange));

  return { mode, isDark, select };
}
