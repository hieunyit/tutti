/*
 * Copyright © 2026 Clyso GmbH
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

export interface TuttiMigrationListResponse {
  migrations: TuttiMigrationItem[];
}

export interface TuttiMigrationItem {
  fromStorage: string;
  toStorage: string;
  started: boolean;
  done: boolean;
  paused: boolean;
  objListed: number;
  objDone: number;
  objBytesListed: number;
  objBytesDone: number;
  buckets: TuttiBucketMigration[];
  startedAt: string;
  doneAt: string;
}

export interface TuttiBucketMigration {
  done: boolean;
  taggingDone: boolean;
  lifecycleDone: boolean;
  policyDone: boolean;
  versioningDone: boolean;
  objListed: number;
  objDone: number;
  objBytesListed: number;
  objBytesDone: number;
  startedAt: string;
  doneAt: string;
  name: string;
}

export interface TuttiMigrationCostListResponse {
  costs: TuttiMigrationCost[];
}

export interface TuttiMigrationCost {
  fromStorage: string;
  toStorage: string;
  status: TuttiMigrationCostStatus;
  startedAt: string;
  buckets: number;
  objects: number;
  bytes: number;
  fromApiCalls: number;
  toApiCalls: string;
}

export enum TuttiMigrationCostStatus {
  NOT_STARTED = 'NOT_STARTED',
  IN_PROGRESS = 'IN_PROGRESS',
  DONE = 'DONE',
}

export interface TuttiProxyCredentials {
  address: string;
  credentials: TuttiCredential[];
}

export interface TuttiCredential {
  alias: string;
  accessKey: string;
  secretKey: string;
}

export interface TuttiStorageListResponse {
  storages: TuttiStorage[];
}

export interface TuttiStorage {
  name: string;
  isMain: boolean;
  address: string;
  provider: StorageProvider;
  credentials: TuttiCredential[];
  /** per-user credential overrides, when the backend reports it */
  userCount?: number;
}

export enum StorageProvider {
  S3 = 'S3',
  SWIFT = 'SWIFT',
}

export interface TuttiCompareBucketRequest {
  bucket: string;
  fromStorage: string;
  toStorage: string;
  user: string;
  showMatch: boolean; // set true to get list of matching files (match property) in response
}

export interface TuttiCompareBucketResponse {
  isMatch: true; // true if storage's buckets have the same content
  missFrom: string[]; // list of missing files in 'from storage' bucket
  missTo: string[]; // list of missing files in 'to storage' bucket
  differ: string[]; // list of files with different content
  error: string[]; // list of errors occurred during comparison
  match: string[]; // list matched files in storages bucket. Empty if request parameter showMatch set to false.
}

export interface TuttiMigrationStreamResponse {
  result: TuttiMigrationListResponse;
  error?: {
    code: number;
    message: string;
  };
}

// Replication
export interface TuttiReplicationId {
  /** empty or omitted for a storage-level replication */
  user?: string;
  fromStorage: string;
  toStorage: string;
  fromBucket?: string;
  toBucket?: string;
}

export interface TuttiReplicationOpts {
  agentUrl: string;
}

export interface TuttiReplicationDowntimeOpts {
  startOnInitDone: boolean;
  cron: string;
  startAt: string;
  maxDuration: string;
  maxEventLag: number;
  skipBucketCheck: boolean;
  continueReplication: boolean;
}

export interface TuttiReplicationSwitchInfo {
  lastStatus: string;
  zeroDowntime: boolean;
  multipartTtl: string;
  downtimeOpts: TuttiReplicationDowntimeOpts;
  lastStartedAt: string;
  doneAt: string;
  history: string[];
  replicationId: TuttiReplicationId;
}

export interface TuttiReplication {
  id: TuttiReplicationId;
  idStr: string;
  opts: TuttiReplicationOpts;
  createdAt: string;
  isPaused: boolean;
  isInitDone: boolean;
  initObjListed: string;
  initObjDone: string;
  events: string;
  eventsDone: string;
  eventLag: string;
  hasSwitch: boolean;
  isArchived: boolean;
  archivedAt: string;
  switchInfo: TuttiReplicationSwitchInfo;
  replicationType: ReplicationType;
}

export interface TuttiReplicationListResponse {
  replications: TuttiReplication[];
}

export interface TuttiAddReplicationsRequest {
  id: TuttiReplicationId;
  opts?: TuttiReplicationOpts;
}

export interface TuttiBucketListRequest {
  /** omitted: buckets are listed with the connection account of the source storage */
  user?: string;
  fromStorage: string;
  toStorage: string;
  showReplicated: boolean;
}

export type TuttiCoveredOwnerKind = 'group' | 'replication';

/** a bucket already covered by a group or another replication of the same edge */
export interface TuttiCoveredBucket {
  bucket: string;
  ownerKind: TuttiCoveredOwnerKind;
  ownerId: string;
}

export interface TuttiBucketListResponse {
  /** never contains covered buckets */
  buckets: string[];
  replicatedBuckets: string[];
  coveredBuckets?: TuttiCoveredBucket[];
}

export enum ReplicationStatusFilter {
  ACTIVE = 'ACTIVE',
  PAUSED = 'PAUSED',
  INITIAL_IN_PROGRESS = 'INITIAL_IN_PROGRESS',
  INITIAL_DONE = 'INITIAL_DONE',
  LIVE_BEHIND = 'LIVE_BEHIND',
  LIVE_UP_TO_DATE = 'LIVE_UP_TO_DATE',
}

export enum AddReplicationStepName {
  'FROM_STORAGE' = 1,
  'TO_STORAGE' = 2,
  'BUCKETS' = 3,
}

export enum ReplicationType {
  'USER' = 'User Replication',
  'BUCKET' = 'Bucket Replication',
}

export enum RoutingPolicyTypes {
  USER = 'User',
  BUCKET = 'Bucket',
}

export interface UserRoutingPolicy {
  user: string;
  toStorage: string;
  isBlocked: boolean;
}

export interface BucketRoutingPolicy extends UserRoutingPolicy {
  bucket: string;
}

export interface RoutingPolicy extends BucketRoutingPolicy {
  id: string;
  type: RoutingPolicyTypes;
}

export interface RoutingPoliciesRequestFilters {
  user: string;
  toStorage: string;
  isBlocked: boolean;
  bucket: string;
}

export interface RoutingPolicyListRequest {
  hideUserRoutings: boolean;
  hideBucketRoutings: boolean;
  filter?: RoutingPoliciesRequestFilters;
}

export interface RoutingPolicyListResponse {
  main: string;
  userRoutings: UserRoutingPolicy[];
  bucketRoutings: BucketRoutingPolicy[];
}

export enum RoutingPolicyStatusFilter {
  ALLOWED = 0,
  BLOCKED = 1,
}

export interface RoutingPolicyEditRequest {
  user: string;
  bucket: string | null;
}

export interface RoutingPolicyAddRequest {
  user: string;
  toStorage: string | null;
  bucket: string | null;
}

export interface S3Credential {
  accessKey: string;
  secretKey: string;
}

export interface SwiftCredential {
  username: string;
  password: string;
  domainName: string;
  tenantName: string;
}

export interface UserCredentialSetRequest {
  storage: string;
  user: string;
  s3Cred?: S3Credential;
  swiftCred?: SwiftCredential;
}

export interface DiffReportLocation {
  storage: string;
  bucket: string;
}

export interface DiffReport {
  locations: DiffReportLocation[];
  queued: string;
  completed: string;
  ready: boolean;
  consistent: boolean;
  versioned: boolean;
  ignoreSizes: boolean;
  ignoreEtags: boolean;
  fixQueued: string;
  fixCompleted: string;
  fixReady: boolean;
}

export interface DiffReportListResponse {
  checks: DiffReport[];
}

export enum DiffReportStatusFilter {
  CHECKING = 'CHECKING',
  CONSISTENT = 'CONSISTENT',
  INCONSISTENT = 'INCONSISTENT',
}

export interface DiffReportDeleteRequest {
  locations: DiffReportLocation[];
}

export type DiffReportRestartRequest = DiffReportDeleteRequest;

export interface AddDiffReportRequest {
  locations: DiffReportLocation[];
  user: string;
  checkOnlyLastVersions: boolean;
  ignoreEtags: boolean;
  ignoreSizes: boolean;
}

export enum AddDiffReportStepName {
  'FROM_STORAGE_BUCKET' = 1,
  'TO_STORAGE_BUCKET' = 2,
  'USER' = 3,
  'SETTINGS' = 4,
}

export interface DiffReportEntry {
  object: string;
  versionIdx: string;
  size: string;
  etag: string;
  storageEntries: DiffReportStorageEntry[];
}

export interface DiffReportStorageEntry {
  storage: string;
  versionId: string;
  bucket: string;
}

export interface DiffReportEntriesRequest {
  locations: DiffReportLocation[];
  cursor?: string;
  pageSize?: string;
}

export interface DiffReportEntriesResponse {
  entries: DiffReportEntry[];
  cursor: string;
}

export interface DiffReportDetailRequest {
  locations: DiffReportLocation[];
}

export interface DiffReportDetailResponse {
  check: DiffReport;
}
