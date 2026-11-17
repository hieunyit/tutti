# Lifecycle operations

Lifecycle expiration deletes data. Before enabling a rule, check the storage, bucket, prefix, expiration period, schedule, and a dry run. The rule runs against objects in the selected bucket whose keys begin with the exact prefix; spaces in the prefix are significant.

The executor requires a bucket with versioning **Enabled** and permission to read its versioning configuration and list object versions. For each expired current object, it deletes only the immutable version it observed. An overwrite that occurs after listing is therefore not deleted by that operation. An expired object is skipped and the run reports an error when its current version has no immutable version ID, or when it has any older live version: deleting the current version could otherwise expose older data. Unversioned or suspended buckets fail closed. Make sure the storage implements S3 version listing and version-specific deletion correctly.

Dry run lists current objects and estimates expiration using their last-modified time and size. It does not check for older versions, version IDs, deletion permissions, or bucket versioning, so its count and bytes can exceed what an actual execution can safely delete. Audit logs and the run's deleted count record successful deletions; a run with any failures returns an error and is scheduled for retry after ten minutes. Investigate reported failures and resolve protected versions deliberately before retrying.

## Upgrade and recovery

Before upgrading to the fenced lifecycle lease implementation, stop and drain **all** old lifecycle schedulers and in-flight lifecycle runs. Older executors do not honor the new lease or safe deletion protocol, so they must not overlap with upgraded executors. Apply the database migrations, deploy the new binaries to all lifecycle workers, then resume scheduling. The lease is renewed during execution; if renewal fails or the lease is lost, the worker cancels its work. Completion is accepted only from the current lease owner. A request already accepted by the storage server may finish despite cancellation, so check audit and storage state when recovering an interrupted run.
