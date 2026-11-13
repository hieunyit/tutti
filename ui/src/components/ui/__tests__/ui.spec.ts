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

import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import StatusTag from '../StatusTag.vue';
import Callout from '../Callout.vue';
import {
  EMPTY_VALUE,
  calloutRole,
  mapStatusToVariant,
  orDash,
} from '../status';

describe('mapStatusToVariant', () => {
  it.each([
    ['healthy', 'success'],
    ['Available', 'success'],
    ['up to date', 'success'],
    ['paused', 'warning'],
    ['Behind', 'warning'],
    ['unavailable', 'danger'],
    ['failed', 'danger'],
    ['blocked', 'danger'],
    ['syncing', 'info'],
    ['initial-sync', 'info'],
    ['disabled', 'neutral'],
    ['something else', 'neutral'],
  ])('maps %s to %s', (status, variant) => {
    expect(mapStatusToVariant(status)).toBe(variant);
  });

  it('is neutral for empty values', () => {
    expect(mapStatusToVariant(null)).toBe('neutral');
    expect(mapStatusToVariant(undefined)).toBe('neutral');
    expect(mapStatusToVariant('')).toBe('neutral');
  });
});

describe('orDash', () => {
  it('shows an en dash for empty values but keeps zero', () => {
    expect(orDash(null)).toBe(EMPTY_VALUE);
    expect(orDash(undefined)).toBe(EMPTY_VALUE);
    expect(orDash('')).toBe(EMPTY_VALUE);
    expect(orDash(0)).toBe('0');
    expect(orDash('x')).toBe('x');
  });
});

describe('calloutRole', () => {
  it('uses alert for danger only', () => {
    expect(calloutRole('danger')).toBe('alert');
    expect(calloutRole('warning')).toBe('status');
    expect(calloutRole('info')).toBe('status');
  });
});

describe('StatusTag', () => {
  it('renders variant class, dot and label', () => {
    const wrapper = mount(StatusTag, {
      props: { variant: 'danger', label: 'Unavailable' },
    });

    expect(wrapper.classes()).toContain('tt-status--danger');
    expect(wrapper.find('.tt-status__dot').exists()).toBe(true);
    expect(wrapper.text()).toBe('Unavailable');
  });

  it('defaults to neutral, supports the slot and hiding the dot', () => {
    const wrapper = mount(StatusTag, {
      props: { noDot: true },
      slots: { default: 'Disabled' },
    });

    expect(wrapper.classes()).toContain('tt-status--neutral');
    expect(wrapper.find('.tt-status__dot').exists()).toBe(false);
    expect(wrapper.text()).toBe('Disabled');
  });
});

describe('Callout', () => {
  it('sets role=alert for danger and status otherwise', () => {
    expect(
      mount(Callout, { props: { tone: 'danger' } }).attributes('role'),
    ).toBe('alert');
    expect(
      mount(Callout, { props: { tone: 'warning' } }).attributes('role'),
    ).toBe('status');
  });
});
