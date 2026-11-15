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

// The OpenAPI spec is a repository file outside ui/. It is bundled at build
// time as one lazy chunk. When the UI is built from a context that only
// contains ui/ (the ui/Dockerfile), the glob matches nothing and the API
// reference shows a "not bundled" message instead of failing the build.

const specModules = import.meta.glob(
  '../../../../../proto/gen/openapi/tutti/tutti.swagger.json',
  { import: 'default' },
) as Record<string, () => Promise<unknown>>;

/** The bundled OpenAPI document, null when not bundled. */
export function loadSpec(): Promise<Record<string, unknown>> | null {
  const loader = Object.values(specModules)[0];

  return loader ? (loader() as Promise<Record<string, unknown>>) : null;
}
