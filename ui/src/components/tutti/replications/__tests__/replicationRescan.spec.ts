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
import { describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { AxiosError } from 'axios';
import ReplicationRescanLine from '../ReplicationRescanLine.vue';
import ReplicationRescanRow from '../ReplicationRescanRow.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));

const WriteGate = { template: '<div><slot /></div>' };
const base = { isInitDone: true, isPaused: false };

describe('ReplicationRescanLine (list)', () => {
  const mountLine = (replication: object) =>
    mount(ReplicationRescanLine, {
      props: { replication: replication as never },
      global: { plugins: [kitI18n()] },
    });

  it('says nothing when the rescan is off', () => {
    expect(mountLine(base).text()).toBe('');
  });

  it('shows the interval, and Scanning while a scan runs', () => {
    expect(mountLine({ ...base, rescanInterval: '3600s' }).text()).toBe(
      'Rescans every 1 h',
    );
    expect(
      mountLine({
        ...base,
        rescanInterval: '900s',
        rescanRunning: true,
      }).text(),
    ).toContain('Scanning…');
  });
});

describe('ReplicationRescanRow (detail)', () => {
  function mountRow(
    replication: object,
    sync = vi.fn().mockResolvedValue(undefined),
    changeInterval = vi.fn().mockResolvedValue(undefined),
  ) {
    const wrapper = mount(ReplicationRescanRow, {
      props: { replication: replication as never, sync, changeInterval },
      global: { plugins: [kitI18n()], stubs: { WriteGate } },
      attachTo: document.body,
    });

    return { wrapper, sync, changeInterval };
  }
  const button = (w: ReturnType<typeof mountRow>['wrapper'], re: RegExp) =>
    w.findAll('button').find((b) => re.test(b.text()))!;

  it('shows interval, last scan with count and next scan', () => {
    const { wrapper } = mountRow({
      ...base,
      rescanInterval: '3600s',
      lastRescanAt: new Date().toISOString(),
      lastRescanObjects: '12',
      nextRescanAt: new Date().toISOString(),
    });

    expect(wrapper.text()).toContain('Every 1 h');
    expect(wrapper.text()).toContain('12 objects copied');
    expect(wrapper.text()).toContain('Next scan');
  });

  it('shows Off and the last error in a warning', () => {
    const { wrapper } = mountRow({ ...base, lastRescanError: 'access denied' });

    expect(wrapper.text()).toContain('Off');
    expect(wrapper.find('.rescan-row__warn').text()).toContain(
      'Last scan failed: access denied',
    );
  });

  it.each([
    [{ isPaused: true }, 'Resume the replication'],
    [{ isInitDone: false }, 'initial copy'],
    [{ rescanRunning: true }, 'already running'],
  ])('disables Sync now with a reason (%j)', (extra, reason) => {
    const { wrapper } = mountRow({ ...base, ...extra });

    expect(button(wrapper, /Sync now/).attributes('disabled')).toBeDefined();
    expect(wrapper.text()).toContain(reason);
  });

  it('starts a scan and shows a server error inline', async () => {
    const error = new AxiosError('x', '409', undefined, undefined, {
      status: 409,
      data: { message: 'FailedPrecondition: scan already running' },
    } as never);
    const { wrapper, sync } = mountRow(base, vi.fn().mockRejectedValue(error));

    await button(wrapper, /Sync now/).trigger('click');
    await flushPromises();

    expect(sync).toHaveBeenCalled();
    expect(wrapper.find('[role="alert"]').text()).toBe('scan already running');
  });

  it('drops the Scan started note once the scan has finished', async () => {
    const { wrapper } = mountRow({ ...base, rescanRunning: false });

    await button(wrapper, /Sync now/).trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain('Scan started');

    await wrapper.setProps({
      replication: { ...base, rescanRunning: true } as never,
    });
    expect(wrapper.text()).toContain('Scan started');

    await wrapper.setProps({
      replication: { ...base, rescanRunning: false } as never,
    });
    expect(wrapper.text()).not.toContain('Scan started');
  });

  it('changes the interval through the select', async () => {
    const { wrapper, changeInterval } = mountRow({
      ...base,
      rescanInterval: '3600s',
    });

    await button(wrapper, /Change/).trigger('click');
    await button(wrapper, /Save/).trigger('click');
    await flushPromises();

    expect(changeInterval).toHaveBeenCalledWith(3600);
    expect(wrapper.find('[role="status"]').text()).toContain('saved');
  });
});
