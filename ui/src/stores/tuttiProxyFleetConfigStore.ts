/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { TuttiService } from '@/services/TuttiService';
import { ProxiesHelper } from '@/utils/helpers/ProxiesHelper';
import type {
  TuttiInt64,
  TuttiProxyFleetConfig,
  TuttiProxySettingsOverride,
} from '@/utils/types/tutti';

export const DEFAULTS_SCOPE = '__defaults__';

export interface FleetDraft {
  defaults: TuttiProxySettingsOverride;
  groups: Record<string, TuttiProxySettingsOverride>;
}

export interface FleetChange {
  scope: string;
  path: string;
  before: unknown;
  after: unknown;
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value ?? {})) as T;
}

function toDraft(config: TuttiProxyFleetConfig | null): FleetDraft {
  return {
    defaults: clone(config?.defaults ?? {}),
    groups: clone(config?.groups ?? {}),
  };
}

function flatten(
  value: unknown,
  prefix: string,
  out: Map<string, unknown>,
): Map<string, unknown> {
  if (value !== null && typeof value === 'object') {
    Object.entries(value as Record<string, unknown>).forEach(([key, child]) =>
      flatten(child, prefix ? `${prefix}.${key}` : key, out),
    );
  } else if (value !== undefined) {
    out.set(prefix, value);
  }

  return out;
}

/** Field-by-field differences between two override objects of one scope. */
export function diffOverride(
  scope: string,
  before: TuttiProxySettingsOverride | undefined,
  after: TuttiProxySettingsOverride | undefined,
): FleetChange[] {
  const a = flatten(before ?? {}, '', new Map());
  const b = flatten(after ?? {}, '', new Map());
  const paths = Array.from(new Set([...a.keys(), ...b.keys()])).sort();

  return paths
    .filter((path) => a.get(path) !== b.get(path))
    .map((path) => ({ scope, path, before: a.get(path), after: b.get(path) }));
}

export const useTuttiProxyFleetConfigStore = defineStore(
  'tuttiProxyFleetConfig',
  () => {
    const config = ref<TuttiProxyFleetConfig | null>(null);
    const draft = ref<FleetDraft>(toDraft(null));
    const loading = ref(false);
    const loadError = ref(false);
    const saving = ref(false);
    const saveError = ref(false);
    const conflict = ref(false);
    const saved = ref(false);

    const version = computed<TuttiInt64>(() => config.value?.version ?? 0);
    const original = computed(() => toDraft(config.value));
    const dirty = computed(
      () => JSON.stringify(draft.value) !== JSON.stringify(original.value),
    );
    const groupNames = computed(() => Object.keys(draft.value.groups).sort());
    const changes = computed<FleetChange[]>(() => {
      const out = diffOverride(
        DEFAULTS_SCOPE,
        original.value.defaults,
        draft.value.defaults,
      );
      const scopes = new Set([
        ...Object.keys(original.value.groups),
        ...Object.keys(draft.value.groups),
      ]);

      Array.from(scopes)
        .sort()
        .forEach((scope) =>
          out.push(
            ...diffOverride(
              scope,
              original.value.groups[scope],
              draft.value.groups[scope],
            ),
          ),
        );

      return out;
    });

    function scopeDraft(scope: string): TuttiProxySettingsOverride {
      return scope === DEFAULTS_SCOPE
        ? draft.value.defaults
        : (draft.value.groups[scope] ?? {});
    }

    function getField(scope: string, path: string[]): unknown {
      return path.reduce<unknown>(
        (node, key) =>
          node && typeof node === 'object'
            ? (node as Record<string, unknown>)[key]
            : undefined,
        scopeDraft(scope),
      );
    }

    /** Set a nested value; undefined (or empty string) removes it and prunes empty parents. */
    function setField(scope: string, path: string[], value: unknown) {
      const root = scopeDraft(scope) as Record<string, unknown>;
      const remove = value === undefined || value === null || value === '';
      const chain: Record<string, unknown>[] = [root];
      let node = root;

      for (const key of path.slice(0, -1)) {
        const next = node[key];

        if (next === undefined || next === null || typeof next !== 'object') {
          if (remove) return;

          node[key] = {};
        }

        node = node[key] as Record<string, unknown>;
        chain.push(node);
      }

      const last = path[path.length - 1] as string;

      if (remove) delete node[last];
      else node[last] = value;

      for (let i = chain.length - 1; i > 0; i -= 1) {
        if (Object.keys(chain[i] as object).length > 0) break;

        delete (chain[i - 1] as Record<string, unknown>)[path[i - 1] as string];
      }
    }

    function addGroup(name: string) {
      const trimmed = name.trim();

      if (trimmed && !draft.value.groups[trimmed])
        draft.value.groups[trimmed] = {};
    }

    function removeGroup(name: string) {
      delete draft.value.groups[name];
    }

    function hydrate(next: TuttiProxyFleetConfig) {
      config.value = next;
      draft.value = toDraft(next);
    }

    async function load() {
      loading.value = true;
      conflict.value = false;

      try {
        hydrate(await TuttiService.getProxyFleetConfig());
        loadError.value = false;
      } catch {
        loadError.value = true;
      } finally {
        loading.value = false;
      }
    }

    function reset() {
      draft.value = toDraft(config.value);
      conflict.value = false;
      saveError.value = false;
      saved.value = false;
    }

    async function save() {
      saving.value = true;
      saved.value = false;
      saveError.value = false;

      try {
        hydrate(
          await TuttiService.setProxyFleetConfig({
            defaults: draft.value.defaults,
            groups: draft.value.groups,
            // optimistic concurrency: only the version this editor loaded may be overwritten
            expectedVersion: version.value,
          }),
        );
        conflict.value = false;
        saved.value = true;
      } catch (e) {
        if (ProxiesHelper.isFleetConflict(e)) conflict.value = true;
        else saveError.value = true;
      } finally {
        saving.value = false;
      }
    }

    return {
      config,
      draft,
      loading,
      loadError,
      saving,
      saveError,
      conflict,
      saved,
      version,
      dirty,
      groupNames,
      changes,
      getField,
      setField,
      addGroup,
      removeGroup,
      load,
      reset,
      save,
    };
  },
);
