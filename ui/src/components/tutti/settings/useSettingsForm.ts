/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import messages from './i18nSettings';
import { RESTART_FIELDS } from './settingsMeta';
import { useTuttiSystemSettingsStore } from '@/stores/tuttiSystemSettingsStore';
import type { TuttiWorkerTuningSources } from '@/utils/types/tutti';

/** Shared plumbing of the settings tabs: draft, validation errors, field attributes, badges. */
export function useSettingsForm() {
  const { t } = useI18n({ messages });
  const store = useTuttiSystemSettingsStore();
  const errors = computed(() => store.validationErrors);
  const draft = computed(() => store.draft);

  function err(key: string): string | undefined {
    return errors.value[key] ? t(errors.value[key]!) : undefined;
  }

  function restart(key: string): string | undefined {
    return RESTART_FIELDS.has(key) ? t('restartRequired') : undefined;
  }

  function inputProps(key: string) {
    return {
      id: `settings-${key}`,
      'aria-describedby': `settings-${key}-hint`,
      'aria-invalid': Boolean(errors.value[key]),
    };
  }

  const status = (key: string) => (errors.value[key] ? 'error' : undefined);

  /** "Auto" badge: the saved value is unset, so the worker uses the hardware-derived one. */
  function autoLabel(key: keyof TuttiWorkerTuningSources): string | undefined {
    return store.settings?.tuningSources?.[key] === 'auto'
      ? t('autoBadge')
      : undefined;
  }

  return { t, store, draft, err, restart, inputProps, status, autoLabel };
}
