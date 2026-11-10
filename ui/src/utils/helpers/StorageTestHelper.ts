/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import type {
  RuntimeStorage,
  StorageTestFeatures,
  StorageTestOptions,
  StorageTestResult,
  StorageTestStep,
} from '@/utils/types/runtimeStorage';

export type StepStatus = 'pass' | 'fail' | 'warn' | 'skip';

export interface StepMeta {
  status: StepStatus;
  variant: 'success' | 'danger' | 'warning' | 'neutral';
  /** one character icon, always paired with a text label */
  icon: string;
}

export interface StepCounts {
  pass: number;
  fail: number;
  warn: number;
  skip: number;
}

/** Where the "save" button of a storage form stands after a test. */
export type SaveGate = 'untested' | 'ok' | 'blocked' | 'force';

export const DEFAULT_FEATURES: StorageTestFeatures = {
  versioning: true,
  tagging: false,
  acl: false,
  lifecycle: false,
  policy: false,
};

/** Form state of the account step; converted to API options by `toOptions`. */
export interface TestOptionsForm {
  features: StorageTestFeatures;
  probeBucketPrefix: string;
  probeBucket: string;
  throughput: boolean;
  /** only for the generated policy, never sent */
  policyPrefix: string;
}

const META: Record<StepStatus, StepMeta> = {
  pass: { status: 'pass', variant: 'success', icon: '✓' },
  fail: { status: 'fail', variant: 'danger', icon: '✕' },
  warn: { status: 'warn', variant: 'warning', icon: '!' },
  skip: { status: 'skip', variant: 'neutral', icon: '–' },
};

export abstract class StorageTestHelper {
  static emptyOptions(): TestOptionsForm {
    return {
      features: { ...DEFAULT_FEATURES },
      probeBucketPrefix: '',
      probeBucket: '',
      throughput: false,
      policyPrefix: '',
    };
  }

  /** Empty fields are omitted so the backend defaults apply. */
  static toOptions(form: TestOptionsForm): StorageTestOptions {
    const options: StorageTestOptions = { features: { ...form.features } };

    if (form.probeBucketPrefix.trim())
      options.probeBucketPrefix = form.probeBucketPrefix.trim();

    if (form.probeBucket.trim()) options.probeBucket = form.probeBucket.trim();

    if (form.throughput) options.throughput = true;

    return options;
  }

  /** Unknown statuses are shown as skipped, never as passed. */
  static stepMeta(status: string | undefined): StepMeta {
    return META[(status ?? '') as StepStatus] ?? META.skip;
  }

  static orderedSteps(result: StorageTestResult | null | undefined) {
    return [...(result?.steps ?? [])].sort(
      (a, b) => (a.number ?? 0) - (b.number ?? 0),
    );
  }

  static counts(result: StorageTestResult | null | undefined): StepCounts {
    const counts: StepCounts = { pass: 0, fail: 0, warn: 0, skip: 0 };

    (result?.steps ?? []).forEach((step) => {
      counts[StorageTestHelper.stepMeta(step.status).status] += 1;
    });

    return counts;
  }

  static failing(
    result: StorageTestResult | null | undefined,
  ): StorageTestStep[] {
    return StorageTestHelper.orderedSteps(result).filter(
      (step) => step.status === 'fail',
    );
  }

  /** A stored result with failures or warnings, for the list badge. */
  static hasProblem(result: StorageTestResult | null | undefined): boolean {
    if (!result) return false;

    return (
      result.ok === false ||
      StorageTestHelper.counts(result).fail > 0 ||
      result.hasWarnings === true
    );
  }

  static latency(value: number | string | undefined): number | null {
    const number = Number(value);

    return value === undefined || value === '' || !Number.isFinite(number)
      ? null
      : number;
  }

  /**
   * untested: no result for the current settings yet.
   * ok: the test passed (warnings allowed).
   * blocked: failing steps and "save anyway" not ticked.
   * force: failing steps and "save anyway" ticked, a confirmation is needed.
   */
  static saveGate(
    result: StorageTestResult | null | undefined,
    saveAnyway: boolean,
  ): SaveGate {
    if (!result) return 'untested';

    if (result.ok && StorageTestHelper.counts(result).fail === 0) return 'ok';

    return saveAnyway ? 'force' : 'blocked';
  }

  /** `force` is only sent when the user insisted on failing steps. */
  static forceFlag(
    result: StorageTestResult | null | undefined,
    saveAnyway: boolean,
    confirmed: boolean,
  ): boolean {
    return (
      StorageTestHelper.saveGate(result, saveAnyway) === 'force' && confirmed
    );
  }

  static canSave(
    result: StorageTestResult | null | undefined,
    saveAnyway: boolean,
    confirmed: boolean,
  ): boolean {
    const gate = StorageTestHelper.saveGate(result, saveAnyway);

    return gate === 'ok' || (gate === 'force' && confirmed);
  }

  /** The list column: the stored test failed or the proxies report an error. */
  static listWarning(
    storage: Pick<RuntimeStorage, 'lastTest'> | undefined,
    lastError: string | undefined,
  ): 'test' | 'error' | null {
    if (lastError) return 'error';

    return storage && StorageTestHelper.hasProblem(storage.lastTest)
      ? 'test'
      : null;
  }
}
