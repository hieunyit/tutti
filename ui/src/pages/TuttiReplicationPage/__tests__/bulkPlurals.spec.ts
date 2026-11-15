import { createI18n } from 'vue-i18n';
import { describe, expect, it, vi } from 'vitest';
import i18nReplicationHub from '../i18nReplicationHub';
import i18nReplications from '@/components/tutti/replications/i18nReplications';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  I18N_DEFAULT_LOCALE: 'en',
}));

type PluralT = (key: string, named: object, plural: number) => string;

function makeT(messages: unknown, locale: string): PluralT {
  const { global } = createI18n({
    legacy: false,
    locale,
    messages: messages as Record<string, never>,
  });

  return global.t as unknown as PluralT;
}

describe('bulk action wording follows the count', () => {
  it('uses the singular for one replication and the plural otherwise', () => {
    const t = makeT(i18nReplicationHub, 'en');

    expect(t('deleteSelected', { n: 1 }, 1)).toBe('Delete 1 replication');
    expect(t('deleteSelected', { n: 3 }, 3)).toBe('Delete 3 replications');
    expect(t('deleteReports', { n: 1 }, 1)).toBe('Delete 1 report');
  });

  it('words the bulk toasts for one and many', () => {
    const t = makeT(i18nReplications, 'en');

    expect(t('pauseSelectedSuccessContent', { total: 1 }, 1)).toBe(
      'The following replication was paused:',
    );
    expect(t('deleteSelectedErrorContent', { total: 2 }, 2)).toBe(
      'An error occurred while deleting the following 2 replications:',
    );
  });
});
