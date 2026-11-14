import { mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';
import ReplicationInitProgress from '../ReplicationInitProgress.vue';
import type { TuttiReplication } from '@/utils/types/tuttiCore';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

function mountWith(r: Partial<TuttiReplication>) {
  return mount(ReplicationInitProgress, {
    props: { replication: r as TuttiReplication },
    global: {
      plugins: [
        createI18n({ legacy: false, locale: 'en', messages: { en: {} } }),
      ],
    },
  });
}

describe('ReplicationInitProgress', () => {
  it('shows copied bytes (incl. in progress), the speed and the object count without total or ETA', () => {
    const text = mountWith({
      initBytesTotal: '10737418240',
      initBytesDone: '209715200',
      initBytesPerSecond: '8388608',
      initEtaSeconds: '600',
      initListingComplete: true,
      initObjDone: '24',
      initObjListed: '5000',
    }).text();

    expect(text).toContain('Copied 200 MB (incl. in progress) · 8.0 MB/s');
    expect(text).toContain('24 / 5000 objects');
    expect(text).not.toContain('10.0 GB');
    expect(text).not.toContain('%');
    expect(text).not.toContain('left');
  });

  it('shows only the object count when bytes are not tracked', () => {
    const text = mountWith({ initObjDone: '1', initObjListed: '2' }).text();

    expect(text).toBe('1 / 2 objects');
  });
});
