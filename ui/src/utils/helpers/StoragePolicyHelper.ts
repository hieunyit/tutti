/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import type { StorageTestFeatures } from '@/utils/types/runtimeStorage';

export interface PolicyStatement {
  Effect: 'Allow';
  Action: string[];
  Resource: string[];
}

export interface StoragePolicy {
  Version: string;
  Statement: PolicyStatement[];
}

const LIST_ONLY = ['s3:ListAllMyBuckets'];
const LOCATION = 's3:GetBucketLocation';
const BUCKET = [
  's3:CreateBucket',
  's3:DeleteBucket',
  's3:ListBucket',
  's3:ListBucketVersions',
  's3:ListBucketMultipartUploads',
];
const OBJECT = [
  's3:GetObject',
  's3:PutObject',
  's3:DeleteObject',
  's3:ListMultipartUploadParts',
  's3:AbortMultipartUpload',
];
const VERSIONING_BUCKET = ['s3:GetBucketVersioning', 's3:PutBucketVersioning'];
const VERSIONING_OBJECT = ['s3:GetObjectVersion', 's3:DeleteObjectVersion'];
const TAGGING_BUCKET = ['s3:GetBucketTagging', 's3:PutBucketTagging'];
const TAGGING_OBJECT = ['s3:GetObjectTagging', 's3:PutObjectTagging'];
const ACL_BUCKET = ['s3:GetBucketAcl', 's3:PutBucketAcl'];
const ACL_OBJECT = ['s3:GetObjectAcl', 's3:PutObjectAcl'];
const LIFECYCLE = [
  's3:GetLifecycleConfiguration',
  's3:PutLifecycleConfiguration',
];
const BUCKET_POLICY = [
  's3:GetBucketPolicy',
  's3:PutBucketPolicy',
  's3:DeleteBucketPolicy',
];

export abstract class StoragePolicyHelper {
  /** MinIO rejects policies naming the ACL actions. */
  static supportsAcl(provider: string): boolean {
    return provider !== 'Minio';
  }

  /**
   * The S3 policy for the connection account (scale plan 2.3). `prefix` is the
   * bucket prefix Tutti manages, e.g. "app-"; empty means the whole storage.
   */
  static generate(
    provider: string,
    features: StorageTestFeatures,
    prefix = '',
  ): StoragePolicy {
    const acl = features.acl && StoragePolicyHelper.supportsAcl(provider);
    const bucketActions = [
      ...BUCKET,
      ...(features.versioning ? VERSIONING_BUCKET : []),
      ...(features.tagging ? TAGGING_BUCKET : []),
      ...(acl ? ACL_BUCKET : []),
      ...(features.lifecycle ? LIFECYCLE : []),
      ...(features.policy ? BUCKET_POLICY : []),
    ];
    const objectActions = [
      ...OBJECT.slice(0, 3),
      ...(features.versioning ? VERSIONING_OBJECT : []),
      ...OBJECT.slice(3),
      ...(features.tagging ? TAGGING_OBJECT : []),
      ...(acl ? ACL_OBJECT : []),
    ];
    const scope = prefix ? `${prefix}*` : '*';
    const all = ['arn:aws:s3:::*'];

    return {
      Version: '2012-10-17',
      Statement: [
        {
          Effect: 'Allow',
          Action: prefix ? LIST_ONLY : [...LIST_ONLY, LOCATION],
          Resource: all,
        },
        {
          Effect: 'Allow',
          Action: prefix ? [LOCATION, ...bucketActions] : bucketActions,
          Resource: [`arn:aws:s3:::${scope}`],
        },
        {
          Effect: 'Allow',
          Action: objectActions,
          Resource: [`arn:aws:s3:::${scope}/*`],
        },
      ],
    };
  }

  static generateJson(
    provider: string,
    features: StorageTestFeatures,
    prefix = '',
  ): string {
    return JSON.stringify(
      StoragePolicyHelper.generate(provider, features, prefix),
      null,
      2,
    );
  }
}
