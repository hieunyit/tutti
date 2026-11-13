/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { defineStore } from 'pinia';
import { ref } from 'vue';
import { TuttiService } from '@/services/TuttiService';
import type { TuttiInt64, TuttiProxyStatus } from '@/utils/types/tutti';

/** Default refresh of the Fleet tab (spec 4.4: 15 s, can be paused). */
export const PROXIES_REFRESH_INTERVAL_MS = 15_000;

export const useTuttiProxiesStore = defineStore('tuttiProxies', () => {
  const proxies = ref<TuttiProxyStatus[]>([]);
  const configVersion = ref<TuttiInt64>(0);
  const loading = ref(false);
  const error = ref(false);
  const loaded = ref(false);
  const polling = ref(false);
  let timer: ReturnType<typeof setInterval> | null = null;
  let controller: AbortController | null = null;

  async function load() {
    if (controller) return;

    const request = new AbortController();

    controller = request;
    loading.value = true;

    try {
      const result = await TuttiService.getProxies(request.signal);

      if (controller !== request) return;

      proxies.value = result.proxies ?? [];
      configVersion.value = result.configVersion ?? 0;
      error.value = false;
      loaded.value = true;
    } catch {
      if (controller === request) error.value = true;
    } finally {
      if (controller === request) {
        controller = null;
        loading.value = false;
      }
    }
  }

  function stopPolling() {
    if (timer !== null) {
      clearInterval(timer);
      timer = null;
    }

    polling.value = false;
    controller?.abort();
    controller = null;
    loading.value = false;
  }

  function startPolling(ms = PROXIES_REFRESH_INTERVAL_MS) {
    stopPolling();
    polling.value = true;
    load();
    timer = setInterval(load, ms);
  }

  return {
    proxies,
    configVersion,
    loading,
    error,
    loaded,
    polling,
    load,
    startPolling,
    stopPolling,
  };
});
