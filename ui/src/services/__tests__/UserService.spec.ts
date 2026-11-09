/* Copyright © 2026 hieunyit. Licensed under AGPL-3.0. */
import { beforeEach, expect, it, vi } from 'vitest';
import { TuttiService } from '../TuttiService';
import { apiClient } from '@/http';

vi.mock('@/http', () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
}));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(apiClient.post).mockResolvedValue({ data: { ok: true } });
  vi.mocked(apiClient.put).mockResolvedValue({ data: {} });
});

it('uses the protobuf credential envelope when saving user credentials', async () => {
  await TuttiService.updateUserCredentials({
    name: 'alice',
    storage: 'main',
    accessKeyId: 'key',
    secretAccessKey: 'secret',
    verify: true,
  });
  expect(apiClient.put).toHaveBeenCalledWith(
    expect.stringContaining('/tutti/users/credentials'),
    {
      user: 'alice',
      credential: {
        storage: 'main',
        accessKeyId: 'key',
        secretAccessKey: 'secret',
      },
      verify: true,
    },
  );
});

it('tests supplied credentials before a user is created', async () => {
  await TuttiService.testUser({
    name: 'new-user',
    storage: 'main',
    accessKeyId: 'key',
    secretAccessKey: 'secret',
  });
  expect(apiClient.post).toHaveBeenCalledWith(
    expect.stringContaining('/tutti/users/test'),
    {
      user: 'new-user',
      storage: 'main',
      credential: {
        storage: 'main',
        accessKeyId: 'key',
        secretAccessKey: 'secret',
      },
    },
  );
});

it('omits the credential envelope when testing saved credentials', async () => {
  await TuttiService.testUser({ name: 'alice', storage: 'main' });
  expect(apiClient.post).toHaveBeenCalledWith(
    expect.stringContaining('/tutti/users/test'),
    { user: 'alice', storage: 'main' },
  );
});
