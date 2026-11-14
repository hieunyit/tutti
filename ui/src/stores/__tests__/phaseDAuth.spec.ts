/* Copyright © 2026 hieunyit. Licensed under AGPL-3.0. */
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, expect, it, vi } from 'vitest';
import type { RouteLocationNormalized } from 'vue-router';
import { AuthService } from '@/services/AuthService';
import { useAuthStore } from '@/stores/authStore';
import authGuard from '@/router/guards/auth-guard';
import { AuthRole } from '@/utils/types/auth';
import { RouteName } from '@/utils/types/router';

vi.mock('@/services/AuthService', () => ({
  AuthService: {
    me: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
    changePassword: vi.fn(),
  },
}));

beforeEach(() => {
  window.localStorage.clear();
  window.sessionStorage.clear();
  window.sessionStorage.setItem('tutti.auth.token', 'session');
  setActivePinia(createPinia());
  vi.resetAllMocks();
});

const run = (name: RouteName, fullPath: string, meta = {}) =>
  (authGuard as (to: RouteLocationNormalized) => Promise<unknown>)({
    name,
    fullPath,
    meta,
  } as RouteLocationNormalized);

it('forces password change before a requested route and permits only the password screen', async () => {
  vi.mocked(AuthService.me).mockResolvedValue({
    authEnabled: true,
    username: 'invited',
    role: AuthRole.VIEWER,
    source: 'managed',
    mustChangePassword: true,
  });
  expect(await run(RouteName.TUTTI_USERS, '/users?tab=all')).toEqual({
    name: RouteName.TUTTI_CHANGE_PASSWORD,
    query: { redirect: '/users?tab=all' },
  });
  expect(await run(RouteName.TUTTI_CHANGE_PASSWORD, '/change-password')).toBe(
    true,
  );
});

it('does not clear the forced state when changing password fails', async () => {
  const auth = useAuthStore();

  auth.mustChangePassword = true;
  vi.mocked(AuthService.changePassword).mockRejectedValueOnce(
    new Error('wrong password'),
  );
  await expect(
    auth.changePassword('wrong', 'A-new-password-2026'),
  ).rejects.toThrow('wrong password');
  expect(auth.mustChangePassword).toBe(true);
  vi.mocked(AuthService.changePassword).mockResolvedValueOnce(undefined);
  await auth.changePassword('old', 'A-new-password-2026');
  expect(auth.mustChangePassword).toBe(false);
});

it('denies console administration to viewers', async () => {
  vi.mocked(AuthService.me).mockResolvedValue({
    authEnabled: true,
    username: 'viewer',
    role: AuthRole.VIEWER,
    source: 'managed',
  });
  expect(
    await run(RouteName.TUTTI_CONSOLE_USERS, '/console-users', {
      requiresAdmin: true,
    }),
  ).toEqual({ name: RouteName.TUTTI_HOME });
});
