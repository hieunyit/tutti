import { describe, expect, it, vi } from 'vitest';

// Message modules only need the locale enum from the kit; the real package
// pulls in CSS that node cannot load.
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));

type Messages = Record<string, unknown>;
type Bundle = Record<string, Messages | undefined>;

const modules = import.meta.glob<Record<string, unknown>>(
  ['/src/**/i18n*.ts', '/src/i18n/messages/*.ts'],
  { eager: true },
);

/** Modules whose vi block is still incomplete (see report); keep this empty. */
const VI_WHITELIST: string[] = [];

function flatten(obj: unknown, prefix = ''): string[] {
  if (obj === null || typeof obj !== 'object') return [prefix];

  return Object.entries(obj as Messages).flatMap(([key, value]) =>
    flatten(value, prefix ? `${prefix}.${key}` : key),
  );
}

function isBundle(value: unknown): value is Bundle {
  return (
    !!value &&
    typeof value === 'object' &&
    typeof (value as Bundle).en === 'object'
  );
}

function bundlesOf(mod: Record<string, unknown>): [string, Bundle][] {
  return Object.entries(mod).filter((entry): entry is [string, Bundle] =>
    isBundle(entry[1]),
  );
}

function missing(bundle: Bundle, locale: string): string[] {
  const have = new Set(flatten(bundle[locale] ?? {}));

  return flatten(bundle.en).filter((key) => !have.has(key));
}

describe('i18n locale completeness', () => {
  const found = Object.entries(modules).flatMap(([path, mod]) =>
    bundlesOf(mod).map(([name, bundle]) => ({
      id: `${path.replace('/src/', '')}#${name}`,
      path: path.replace('/src/', ''),
      bundle,
    })),
  );

  it('discovers message modules', () => {
    expect(found.length).toBeGreaterThan(15);
  });

  it('reports de gaps (informational)', () => {
    const gaps = found
      .map((f) => [f.id, missing(f.bundle, 'de').length] as const)
      .filter(([, n]) => n > 0);

    // eslint-disable-next-line no-console
    if (gaps.length) console.info('de gaps', gaps);

    expect(Array.isArray(gaps)).toBe(true);
  });

  it('has every en key in vi', () => {
    const gaps = found
      .filter((f) => !VI_WHITELIST.includes(f.path))
      .map((f) => ({ id: f.id, keys: missing(f.bundle, 'vi') }))
      .filter((g) => g.keys.length > 0);

    expect(
      gaps.map((g) => `${g.id}: ${g.keys.length} missing, e.g. ${g.keys[0]}`),
    ).toEqual([]);
  });
});
