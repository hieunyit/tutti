/*
 * Copyright © 2026 hieunyit
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

import type { RouteLocationRaw } from 'vue-router';
import { DiffReportsHelper } from '@/utils/helpers/DiffReportsHelper';
import type { DiffReportLocation } from '@/utils/types/tuttiCore';
import { RouteName } from '@/utils/types/router';

/** "<storage:bucket>~<storage:bucket>": the id of a diff report in the URL. */
export function formatDiffId(from: DiffReportLocation, to: DiffReportLocation) {
  return `${DiffReportsHelper.formatLocationForQuery(from)}~${DiffReportsHelper.formatLocationForQuery(to)}`;
}

/** The two locations of a diff report id, or null when it is not one. */
export function parseDiffId(id: string): DiffReportLocation[] | null {
  const locations = id
    .split('~')
    .map((part) => DiffReportsHelper.parseLocationFromQuery(part))
    .filter((location): location is DiffReportLocation => location !== null);

  return locations.length === 2 ? locations : null;
}

/**
 * Route of a diff report's detail page. The from/to query stays because the
 * detail store reads the locations from it.
 */
export function diffDetailRoute(
  from: DiffReportLocation,
  to: DiffReportLocation,
): RouteLocationRaw {
  return {
    name: RouteName.TUTTI_REPLICATION_DIFF_DETAIL,
    params: { id: formatDiffId(from, to) },
    query: {
      from: DiffReportsHelper.formatLocationForQuery(from),
      to: DiffReportsHelper.formatLocationForQuery(to),
    },
  };
}
