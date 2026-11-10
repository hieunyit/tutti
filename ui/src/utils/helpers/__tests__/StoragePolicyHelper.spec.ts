/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { describe, expect, it } from 'vitest';
import { StoragePolicyHelper } from '../StoragePolicyHelper';
import type { StorageTestFeatures } from '@/utils/types/runtimeStorage';

const none: StorageTestFeatures = {
  versioning: false,
  tagging: false,
  acl: false,
  lifecycle: false,
  policy: false,
};
const all: StorageTestFeatures = {
  versioning: true,
  tagging: true,
  acl: true,
  lifecycle: true,
  policy: true,
};
const actions = (
  provider: string,
  features: StorageTestFeatures,
  prefix = '',
) =>
  StoragePolicyHelper.generate(provider, features, prefix).Statement.flatMap(
    (statement) => statement.Action,
  );

describe('StoragePolicyHelper', () => {
  it('always grants the base set', () => {
    const base = actions('Minio', none);

    expect(base).toEqual(
      expect.arrayContaining([
        's3:ListAllMyBuckets',
        's3:GetBucketLocation',
        's3:CreateBucket',
        's3:DeleteBucket',
        's3:ListBucket',
        's3:GetObject',
        's3:PutObject',
        's3:DeleteObject',
        's3:AbortMultipartUpload',
        's3:ListMultipartUploadParts',
        's3:ListBucketMultipartUploads',
      ]),
    );
    expect(base).not.toContain('s3:PutBucketVersioning');
    expect(base).not.toContain('s3:PutObjectTagging');
  });

  it('never names ACL actions for MinIO, even with the ACL feature on', () => {
    expect(actions('Minio', all).filter((a) => /Acl/.test(a))).toEqual([]);
    expect(StoragePolicyHelper.supportsAcl('Minio')).toBe(false);
  });

  it('adds ACL actions for Ceph and other providers only when enabled', () => {
    ['Ceph', 'Other'].forEach((provider) => {
      expect(actions(provider, all)).toEqual(
        expect.arrayContaining([
          's3:GetBucketAcl',
          's3:PutBucketAcl',
          's3:GetObjectAcl',
          's3:PutObjectAcl',
        ]),
      );
      expect(
        actions(provider, { ...all, acl: false }).filter((a) => /Acl/.test(a)),
      ).toEqual([]);
    });
  });

  it('adds only the actions of the enabled features', () => {
    expect(actions('Ceph', { ...none, versioning: true })).toEqual(
      expect.arrayContaining([
        's3:GetBucketVersioning',
        's3:PutBucketVersioning',
        's3:GetObjectVersion',
        's3:DeleteObjectVersion',
      ]),
    );
    expect(actions('Ceph', { ...none, lifecycle: true })).toContain(
      's3:PutLifecycleConfiguration',
    );
    expect(actions('Ceph', { ...none, policy: true })).toContain(
      's3:DeleteBucketPolicy',
    );
    expect(actions('Ceph', { ...none, tagging: true })).toEqual(
      expect.arrayContaining(['s3:PutBucketTagging', 's3:GetObjectTagging']),
    );
    expect(actions('Ceph', none)).not.toContain('s3:PutLifecycleConfiguration');
  });

  it('scopes the resources to a bucket prefix', () => {
    const policy = StoragePolicyHelper.generate('Minio', none, 'app-');

    expect(policy.Statement[1]!.Resource).toEqual(['arn:aws:s3:::app-*']);
    expect(policy.Statement[2]!.Resource).toEqual(['arn:aws:s3:::app-*/*']);
    expect(policy.Statement[0]!.Resource).toEqual(['arn:aws:s3:::*']);
    expect(policy.Statement[0]!.Action).toEqual(['s3:ListAllMyBuckets']);
    expect(policy.Statement[1]!.Action).toContain('s3:GetBucketLocation');
  });

  it('uses the whole storage and valid JSON by default', () => {
    const json = StoragePolicyHelper.generateJson('Minio', all);
    const policy = JSON.parse(json);

    expect(policy.Version).toBe('2012-10-17');
    expect(policy.Statement[2].Resource).toEqual(['arn:aws:s3:::*/*']);
  });
});
