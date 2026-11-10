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

import { EMPTY_VALUE, type StatusVariant } from '@/components/ui/status';
import type {
  ProtectionDisplayState,
  ProtectionFormError,
  ProtectionGroup,
  ProtectionGroupCreateRequest,
  ProtectionGroupForm,
  ProtectionGroupStatus,
  ProtectionMemberStatus,
} from '@/utils/types/protection';

const ID_FORBIDDEN = /[:/\s]/;
const BUCKET_FORBIDDEN = /[*?[\]:/\s]/;
const NUL = String.fromCharCode(0);

function hasForbidden(value: string, pattern: RegExp): boolean {
  return pattern.test(value) || value.includes(NUL);
}

export type MemberHealth = 'healthy' | 'syncing' | 'lagging' | 'unavailable';

export interface MemberRow {
  storage: string;
  isPrimary: boolean;
  isTarget: boolean;
  excluded: boolean;
  health: MemberHealth;
  lag: number | null;
  initialized: boolean | null;
}

export type SwitchStepId = 'waiting' | 'draining' | 'promoting' | 'resync';
export type SwitchStepStatus = 'done' | 'active' | 'pending';

export abstract class ProtectionGroupHelper {
  /** State word shown to the operator: error and frozen take precedence. */
  static displayState(group: ProtectionGroup): ProtectionDisplayState {
    const state = group.state ?? '';

    if (group.lastError && state !== 'ready') return 'error';

    if (state === 'ready') return 'ready';

    if (group.frozen && state === 'initializing') return 'frozen';

    if (
      state === 'initializing' ||
      state === 'waiting' ||
      state === 'draining' ||
      state === 'promoting'
    ) {
      return state;
    }

    return group.frozen ? 'frozen' : 'unknown';
  }

  static stateVariant(state: ProtectionDisplayState): StatusVariant {
    switch (state) {
      case 'ready':
        return 'success';

      case 'initializing':

      case 'waiting':
        return 'info';

      case 'draining':

      case 'promoting':

      case 'frozen':
        return 'warning';

      case 'error':
        return 'danger';

      default:
        return 'neutral';
    }
  }

  static toNumber(value: string | number | undefined | null): number {
    const n = Number(value ?? 0);

    return Number.isFinite(n) ? n : 0;
  }

  static afterSeconds(after: string | undefined): number | null {
    if (!after) return null;

    const match = /^(\d+(?:\.\d+)?)s$/.exec(after.trim());

    return match ? Number(match[1]) : null;
  }

  /** wanted copies (falls back to member count) */
  static wantedCopies(group: ProtectionGroup): number {
    return group.copies ?? (group.members ?? []).length;
  }

  /** Healthy below wanted (needs the status); null while the status is unknown. */
  static isDegraded(
    group: ProtectionGroup,
    status: ProtectionGroupStatus | undefined,
  ): boolean | null {
    if (!status) return null;

    return (
      ProtectionGroupHelper.toNumber(status.healthyCopies) <
      ProtectionGroupHelper.wantedCopies(group)
    );
  }

  /** A group needs attention when copies are below wanted, it is frozen or in error. */
  static needsAttention(
    group: ProtectionGroup,
    status: ProtectionGroupStatus | undefined,
  ): boolean {
    return (
      group.frozen === true ||
      ProtectionGroupHelper.displayState(group) === 'error' ||
      ProtectionGroupHelper.isDegraded(group, status) === true
    );
  }

  static attentionCount(
    groups: ProtectionGroup[],
    statuses: Record<string, ProtectionGroupStatus>,
  ): number {
    return groups.filter((g) =>
      ProtectionGroupHelper.needsAttention(g, statuses[g.id]),
    ).length;
  }

  static summary(
    groups: ProtectionGroup[],
    statuses: Record<string, ProtectionGroupStatus>,
  ) {
    let healthy = 0;
    let degraded = 0;
    let frozen = 0;

    for (const g of groups) {
      const status = statuses[g.id];
      const isDegraded = ProtectionGroupHelper.isDegraded(g, status);

      if (g.frozen) frozen += 1;

      if (isDegraded === true) degraded += 1;
      else if (isDegraded === false && g.state === 'ready') healthy += 1;
    }

    return { total: groups.length, healthy, degraded, frozen };
  }

  static maxLag(status: ProtectionGroupStatus | undefined): number | null {
    const members = status?.members;

    if (!members || members.length === 0) return null;

    return Math.max(...members.map((m) => m.eventLag ?? 0));
  }

  static scopeKind(group: ProtectionGroup): 'user' | 'buckets' {
    return (group.buckets ?? []).length > 0 ? 'buckets' : 'user';
  }

  static memberHealth(m: ProtectionMemberStatus | undefined): MemberHealth {
    if (!m || m.available === false) return 'unavailable';

    if (m.initialized === false) return 'syncing';

    if (m.current) return 'healthy';

    return 'lagging';
  }

  /** Members in the stored order with role/health; the primary is flagged, not moved. */
  static memberRows(
    group: ProtectionGroup,
    status: ProtectionGroupStatus | undefined,
  ): MemberRow[] {
    return (group.members ?? []).map((storage) => {
      const m = status?.members?.find((item) => item.storage === storage);

      return {
        storage,
        isPrimary: storage === group.primary,
        isTarget: !!group.target && storage === group.target,
        excluded: (group.quarantined ?? []).includes(storage),
        health: status ? ProtectionGroupHelper.memberHealth(m) : 'lagging',
        lag: m ? (m.eventLag ?? 0) : null,
        initialized: m ? m.initialized !== false : null,
      };
    });
  }

  /** Switch candidates: members that are neither primary nor quarantined. */
  static switchTargets(group: ProtectionGroup): string[] {
    return (group.members ?? []).filter(
      (m) => m !== group.primary && !(group.quarantined ?? []).includes(m),
    );
  }

  static canSwitch(group: ProtectionGroup): boolean {
    return (
      group.state === 'ready' &&
      !group.target &&
      ProtectionGroupHelper.switchTargets(group).length > 0
    );
  }

  /** A pending, not yet committed switch can be cancelled. */
  static canAbort(group: ProtectionGroup): boolean {
    return !!group.target && group.state !== 'promoting';
  }

  static canDelete(group: ProtectionGroup): boolean {
    return group.state === 'ready';
  }

  static canEdit(group: ProtectionGroup): boolean {
    return group.state === 'ready';
  }

  /** Progress of a planned switch derived from the group state. */
  static switchSteps(
    group: ProtectionGroup,
  ): { id: SwitchStepId; status: SwitchStepStatus }[] {
    const order: SwitchStepId[] = [
      'waiting',
      'draining',
      'promoting',
      'resync',
    ];
    let current = -1;

    if (group.target) {
      if (group.state === 'waiting') current = 0;
      else if (group.state === 'draining') current = 1;
      else if (group.state === 'promoting') current = 2;
      else current = 0;
    } else if (group.state === 'initializing' && group.frozen) {
      current = 3;
    }

    return order.map((id, index) => ({
      id,
      status:
        current < 0
          ? 'pending'
          : index < current
            ? 'done'
            : index === current
              ? 'active'
              : 'pending',
    }));
  }

  // ---- members ordering ------------------------------------------------

  static moveMember(members: string[], index: number, dir: -1 | 1): string[] {
    const to = index + dir;

    if (
      index < 0 ||
      index >= members.length ||
      to < 0 ||
      to >= members.length
    ) {
      return members;
    }

    const result = [...members];
    const moved = result[index] as string;

    result[index] = result[to] as string;
    result[to] = moved;

    return result;
  }

  static addMember(members: string[], storage: string): string[] {
    return members.includes(storage) ? members : [...members, storage];
  }

  static removeMember(members: string[], storage: string): string[] {
    return members.filter((m) => m !== storage);
  }

  // ---- form / payloads -------------------------------------------------

  static emptyForm(): ProtectionGroupForm {
    return {
      id: '',
      user: null,
      buckets: [],
      members: [],
      copies: 2,
      maxEventLag: null,
      mode: 'manual',
      afterSeconds: null,
      electorate: [],
      maxSwitchesPerHour: null,
      adoptExisting: false,
    };
  }

  static groupToForm(group: ProtectionGroup): ProtectionGroupForm {
    return {
      id: group.id,
      user: group.user ?? null,
      buckets: [...(group.buckets ?? [])],
      members: [...(group.members ?? [])],
      copies: group.copies ?? 2,
      maxEventLag: group.maxEventLag ?? null,
      mode: group.mode === 'auto' ? 'auto' : 'manual',
      afterSeconds: ProtectionGroupHelper.afterSeconds(group.after),
      electorate: [...(group.electorate ?? [])],
      maxSwitchesPerHour: group.maxSwitchesPerHour ?? null,
      adoptExisting: false,
    };
  }

  /** Keeps copies within 2..members when the member list changes. */
  static clampCopies(copies: number, memberCount: number): number {
    return Math.min(Math.max(copies, 2), Math.max(memberCount, 2));
  }

  /** Mirrors pkg/protectiongroups Group.Validate; the backend stays authoritative. */
  static validate(
    form: ProtectionGroupForm,
    isEdit: boolean,
  ): ProtectionFormError[] {
    const errors: ProtectionFormError[] = [];

    if (!isEdit) {
      if (!form.id.trim()) errors.push('ID_REQUIRED');
      else if (hasForbidden(form.id.trim(), ID_FORBIDDEN))
        errors.push('ID_INVALID');

      if (form.members.length < 2) errors.push('MEMBERS_MIN');

      if (
        new Set(form.buckets).size !== form.buckets.length ||
        form.buckets.some((b) => !b || hasForbidden(b, BUCKET_FORBIDDEN))
      ) {
        errors.push('BUCKET_INVALID');
      }
    }

    const memberCount = form.members.length;

    if (form.copies < 2 || form.copies > Math.max(memberCount, 2)) {
      errors.push('COPIES_RANGE');
    }

    if (form.maxEventLag !== null && form.maxEventLag < 0) {
      errors.push('LAG_NEGATIVE');
    }

    if (form.afterSeconds !== null && form.afterSeconds < 0) {
      errors.push('AFTER_NEGATIVE');
    }

    if (form.maxSwitchesPerHour !== null && form.maxSwitchesPerHour < 1) {
      errors.push('SWITCHES_POSITIVE');
    }

    if (form.mode === 'auto' && new Set(form.electorate).size < 3) {
      errors.push('ELECTORATE_MIN');
    }

    return errors;
  }

  /** Common mutable fields; unset numbers are omitted so the backend defaults apply. */
  private static mutable(form: ProtectionGroupForm): Partial<ProtectionGroup> {
    const out: Partial<ProtectionGroup> = {
      copies: form.copies,
      mode: form.mode,
      electorate: form.mode === 'auto' ? [...form.electorate] : [],
    };

    if (form.maxEventLag !== null) out.maxEventLag = form.maxEventLag;

    if (form.afterSeconds !== null) out.after = `${form.afterSeconds}s`;

    if (form.maxSwitchesPerHour !== null) {
      out.maxSwitchesPerHour = form.maxSwitchesPerHour;
    }

    return out;
  }

  static buildCreateRequest(
    form: ProtectionGroupForm,
  ): ProtectionGroupCreateRequest {
    return {
      group: {
        id: form.id.trim(),
        user: form.user ?? '',
        buckets: [...form.buckets],
        members: [...form.members],
        ...ProtectionGroupHelper.mutable(form),
      } as ProtectionGroup,
      adoptExisting: form.adoptExisting,
    };
  }

  /** Update echoes the full stored group (scope and members unchanged) and its revision. */
  static buildUpdateRequest(
    current: ProtectionGroup,
    form: ProtectionGroupForm,
  ): ProtectionGroup {
    return {
      id: current.id,
      user: current.user,
      buckets: current.buckets ?? [],
      members: current.members ?? [],
      revision: current.revision,
      ...ProtectionGroupHelper.mutable(form),
    } as ProtectionGroup;
  }

  // ---- writers ---------------------------------------------------------

  /** "<owner>:<generation>:<instance>:<admittedUnixMs>"; owner may contain ':'. */
  static parseWriter(record: string): {
    owner: string;
    generation: number;
    instance: string;
    admittedAt: Date | null;
  } {
    const parts = record.split(':');
    const n = parts.length;

    if (n < 4) {
      return { owner: record, generation: 0, instance: '', admittedAt: null };
    }

    const ms = Number(parts[n - 1]);

    return {
      owner: parts.slice(0, n - 3).join(':'),
      generation: Number(parts[n - 3]) || 0,
      instance: parts[n - 2] ?? '',
      admittedAt: Number.isFinite(ms) ? new Date(ms) : null,
    };
  }

  /** Newest history entry first (the API returns oldest first). */
  static historyNewestFirst(group: ProtectionGroup) {
    return [...(group.history ?? [])].reverse();
  }

  static lagText(lag: number | null | undefined): string {
    return lag === null || lag === undefined ? EMPTY_VALUE : String(lag);
  }
}
