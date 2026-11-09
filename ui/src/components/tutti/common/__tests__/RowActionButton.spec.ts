import { mount } from '@vue/test-utils';
import { expect, it, vi } from 'vitest';
import RowActionButton from '../RowActionButton.vue';
import { IconName } from '@/utils/types/icon';

vi.mock('@clyso/clyso-ui-kit', () => ({
  CTooltip: { template: '<div><slot name="trigger" /><slot /></div>' },
  CButton: { template: '<button><slot name="icon" /></button>' },
  CIcon: { template: '<span />' },
}));

it('forwards click and accessible trigger attributes to the actual button', async () => {
  const onClick = vi.fn();
  const wrapper = mount(RowActionButton, {
    props: { label: 'Edit policy', icon: IconName.BASE_CREATE, onClick },
    attrs: { 'data-trigger': 'policy' },
  });
  const button = wrapper.get('button');

  expect(wrapper.element.tagName).toBe('SPAN');
  expect(button.attributes('aria-label')).toBe('Edit policy');
  expect(button.attributes('data-trigger')).toBe('policy');
  await button.trigger('click');
  expect(onClick).toHaveBeenCalledTimes(1);
});
