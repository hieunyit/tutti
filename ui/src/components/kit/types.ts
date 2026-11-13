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

/** Status colors: semantic only, never decoration. */
export type StatusTone = 'success' | 'warning' | 'danger' | 'info' | 'neutral';

export type PageState = 'loading' | 'empty' | 'error' | 'ready';

export interface PageShellTab {
  key: string;
  label: string;
  badge?: number;
}

export interface WizardStep {
  key: string;
  label: string;
  valid: boolean;
}

/** An active filter shown as a removable chip in FilterBar. */
export interface FilterChip {
  key: string;
  label: string;
}

export interface StatTrend {
  direction: 'up' | 'down' | 'flat';
  /** e.g. "+3 since yesterday" */
  label: string;
}
