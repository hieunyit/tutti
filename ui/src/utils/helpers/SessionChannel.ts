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

export type SessionMessage =
  | { type: 'refreshed'; expiresAt: number; maxExpiresAt: number }
  | { type: 'logout' };

export interface SessionChannelLike {
  post: (message: SessionMessage) => void;
  subscribe: (listener: (message: SessionMessage) => void) => () => void;
}

const NAME = 'tutti-session';

function isMessage(value: unknown): value is SessionMessage {
  const m = value as Partial<SessionMessage> | null;

  if (!m || typeof m !== 'object') return false;

  if (m.type === 'logout') return true;

  return (
    m.type === 'refreshed' &&
    Number.isFinite((m as { expiresAt?: number }).expiresAt) &&
    Number.isFinite((m as { maxExpiresAt?: number }).maxExpiresAt)
  );
}

/**
 * Tab-to-tab messages: BroadcastChannel('tutti-session'), with a `storage`
 * event fallback. A tab never receives its own messages.
 */
function create(): SessionChannelLike {
  const listeners = new Set<(message: SessionMessage) => void>();
  const emit = (value: unknown) => {
    if (isMessage(value)) listeners.forEach((l) => l(value));
  };

  if (typeof BroadcastChannel !== 'undefined') {
    const channel = new BroadcastChannel(NAME);

    channel.onmessage = (e: MessageEvent) => emit(e.data);

    return {
      post: (message) => channel.postMessage(message),
      subscribe(listener) {
        listeners.add(listener);

        return () => listeners.delete(listener);
      },
    };
  }

  window.addEventListener('storage', (e: StorageEvent) => {
    if (e.key !== NAME || !e.newValue) return;

    try {
      emit(JSON.parse(e.newValue));
    } catch {
      // ignore foreign values
    }
  });

  return {
    post(message) {
      try {
        window.localStorage.setItem(
          NAME,
          JSON.stringify({ ...message, n: Date.now() + Math.random() }),
        );
      } catch {
        // storage unavailable: single-tab behaviour
      }
    },
    subscribe(listener) {
      listeners.add(listener);

      return () => listeners.delete(listener);
    },
  };
}

let instance: SessionChannelLike | null = null;

/** One shared instance, so a tab's own posts never loop back to it. */
export function getSessionChannel(): SessionChannelLike {
  instance ??= create();

  return instance;
}
