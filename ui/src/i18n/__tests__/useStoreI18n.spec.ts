import { effectScope, nextTick } from 'vue';
import { expect, it, vi } from 'vitest';
import { I18nLocale } from '@clyso/clyso-ui-kit';
import { useStoreI18n } from '../useStoreI18n';
import { i18n } from '@/i18n';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));

it('registers store messages under their locale and follows language changes', async () => {
  const initialLocale = i18n.global.locale.value;
  const scope = effectScope();

  try {
    i18n.global.locale.value = I18nLocale.EN;
    const composer = scope.run(() =>
      useStoreI18n({
        messages: {
          en: { title: 'Store title' },
          de: { title: 'Store-Titel' },
        },
      }),
    );

    expect(composer?.t('title')).toBe('Store title');
    i18n.global.locale.value = I18nLocale.DE;
    await nextTick();
    expect(composer?.t('title')).toBe('Store-Titel');
  } finally {
    scope.stop();
    i18n.global.locale.value = initialLocale;
  }
});
