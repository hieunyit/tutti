/*
 * Copyright © 2026 Clyso GmbH
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

import { defineStore } from 'pinia';
import { computed, reactive, toRefs } from 'vue';
import { useRoute } from 'vue-router';
import { usePolling } from '@/utils/composables/usePolling';
import type { DiffReport, DiffReportLocation } from '@/utils/types/tuttiCore';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { DiffReportsHelper } from '@/utils/helpers/DiffReportsHelper';
import { ErrorHelper } from '@/utils/helpers/ErrorHelper';

interface TuttiDiffReportDetailState {
  isLoading: boolean;
  isRestartLoading: boolean;
  hasError: boolean;
  isNotFound: boolean;
  locations: DiffReportLocation[];
  report: DiffReport | null;
}

const POLL_INTERVAL_MS = 5000;

function getInitialState(): TuttiDiffReportDetailState {
  return {
    isLoading: false,
    isRestartLoading: false,
    hasError: false,
    isNotFound: false,
    locations: [],
    report: null,
  };
}

export const useTuttiDiffReportDetailStore = defineStore(
  'tuttiDiffReportDetail',
  () => {
    const state = reactive<TuttiDiffReportDetailState>(getInitialState());
    const polling = usePolling();
    const route = useRoute();

    state.locations = DiffReportsHelper.parseQueryParamsToLocations(
      route.query.from,
      route.query.to,
    );

    const hasFixActivity = computed(() => {
      if (!state.report) return false;

      return (
        !state.report.consistent &&
        (Number(state.report.fixQueued) > 0 ||
          Number(state.report.fixCompleted) > 0 ||
          state.report.fixReady)
      );
    });

    let generation = 0;

    async function getDiffReport(currentGen: number) {
      if (!state.locations) return;

      const res = await TuttiCoreService.getDiffReport({
        locations: state.locations,
      });

      if (currentGen === generation) {
        state.report = res.check;
      }
    }

    async function pollOnce(currentGen: number) {
      try {
        await getDiffReport(currentGen);

        if (currentGen === generation) {
          state.hasError = false;
        }
      } catch (e) {
        if (currentGen !== generation) {
          return;
        }

        if (ErrorHelper.getStatusCode(e) === 404) {
          state.isNotFound = true;
          void polling.stop();

          return;
        }

        if (!state.report) {
          state.hasError = true;
        }

        // Transient error (e.g. 503): the polling loop keeps going
      }
    }

    async function startPolling() {
      stopPolling();
      const currentGen = generation;

      await polling.start(() => pollOnce(currentGen), POLL_INTERVAL_MS);
    }

    function stopPolling() {
      generation++;
      void polling.stop();
    }

    async function initDiffReportDetailsPage() {
      state.locations = DiffReportsHelper.parseQueryParamsToLocations(
        route.query.from,
        route.query.to,
      );

      if (state.locations.length !== 2) {
        return;
      }

      state.isLoading = true;

      try {
        await startPolling();
      } finally {
        state.isLoading = false;
      }
    }

    async function restartDiffReport() {
      state.isRestartLoading = true;

      stopPolling();
      const currentGen = generation;

      try {
        await TuttiCoreService.restartDiffReport({
          locations: state.locations,
        });
      } finally {
        if (currentGen === generation) {
          state.isRestartLoading = false;
          await startPolling();
        }
      }
    }

    function $reset() {
      try {
        stopPolling();
      } finally {
        Object.assign(state, getInitialState());
      }
    }

    return {
      ...toRefs(state),
      initDiffReportDetailsPage,
      restartDiffReport,
      hasFixActivity,
      $reset,
    };
  },
);
