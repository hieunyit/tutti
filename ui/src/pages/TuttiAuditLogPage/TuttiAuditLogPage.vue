<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->

<script setup lang="ts">
  import { CButton, CIcon, I18nLocale } from '@clyso/clyso-ui-kit';
  import { NInput, NModal, NSelect, NTooltip } from 'naive-ui';
  import { computed, onBeforeMount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import ErrorState from '@/components/tutti/common/ErrorState.vue';
  import Callout from '@/components/ui/Callout.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import StatusTag from '@/components/ui/StatusTag.vue';
  import { EMPTY_VALUE, type StatusVariant } from '@/components/ui/status';
  import { AuditLogService } from '@/services/AuditLogService';
  import {
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import {
    AUDIT_LOG_ACTIONS,
    AUDIT_LOG_FAMILIES,
    type AuditLogEntry,
  } from '@/utils/types/audit';
  import { IconName } from '@/utils/types/icon';

  const auditMessages = {
    [I18nLocale.EN]: {
      title: 'Control-Plane Audit Log',
      pageDescription:
        'Transactional history of control-plane configuration changes across storages, identities, routing, replication, failover, and protection groups.',
      refresh: 'Refresh',
      retry: 'Retry',
      applyFilters: 'Apply filters',
      resetFilters: 'Reset',
      errorTitle: 'Audit log could not be loaded',
      errorText:
        'The control plane did not respond or audit log querying is unavailable.',
      backendLabel: 'Control-Plane Backend',
      backendPostgres: 'PostgreSQL (transactional)',
      backendRedis: 'Redis (in-memory / non-durable audit)',
      modeLabel: 'Write Mode',
      modeReadOnly: 'Read-only mode active',
      modeReadWrite: 'Read-write',
      redisBannerTitle: 'Transactional audit log requires PostgreSQL backend',
      redisBannerText:
        'Control-plane state is currently served from Redis. Durable transactional audit logging with before/after snapshots is active when controlPlane.backend is set to "postgres".',
      readOnlyBannerTitle: 'Control plane is in read-only mode',
      readOnlyBannerText:
        'Configuration mutations are currently blocked (controlPlane.readOnly is enabled). Existing audit history remains queryable.',
      filtersTitle: 'Filter audit entries',
      familyLabel: 'Family',
      familyAll: 'All families',
      entityKeyLabel: 'Object / Entity ID',
      entityKeyPlaceholder: 'e.g. main, alice, pg-prod…',
      actorLabel: 'Actor',
      actorPlaceholder: 'e.g. admin, system…',
      actionLabel: 'Action',
      actionAll: 'All actions',
      limitLabel: 'Page size',
      tableTitle: 'Audit entries',
      tableMeta: '{count} entries shown',
      tableMetaCursor: '{count} entries · before #{cursor}',
      emptyTitle: 'No audit log entries found',
      emptyText:
        'No control-plane mutations match the current filters, or no changes have been recorded yet.',
      colId: 'ID',
      colTimestamp: 'Timestamp',
      colActor: 'Actor / Request',
      colAction: 'Action',
      colFamily: 'Family',
      colEntityKey: 'Object ID',
      colDetails: 'Details / Diff',
      requestIdPrefix: 'req:',
      showDiff: 'View diff',
      hideDiff: 'Hide diff',
      openModal: 'Expand modal',
      beforeSnapshot: 'Before (beforeJson)',
      afterSnapshot: 'After (afterJson)',
      noBeforeSnapshot: 'No prior state (created or initial snapshot)',
      noAfterSnapshot: 'No target state (deleted)',
      redactedNote:
        'Secrets and credentials are automatically masked ([REDACTED] / ***) before persistence.',
      changedFields: 'Changed fields',
      noChangedFields: 'No top-level field diff detected',
      paginationPage: 'Page {page}',
      paginationFirst: 'Latest',
      paginationNewer: '← Newer',
      paginationOlder: 'Load older →',
      modalTitle: 'Audit Entry #{id} · {family}/{entityKey}',
      close: 'Close',
      langSwitchVi: 'Tiếng Việt',
      langSwitchEn: 'English',
      'family.storage': 'Storage',
      'family.s3_identity': 'S3 Identity',
      'family.dynamic_credential': 'Dynamic Credential',
      'family.proxy_account': 'Proxy Account',
      'family.access_policy': 'Access Policy',
      'family.console_user': 'Console User',
      'family.route': 'Routing Policy',
      'family.route_block': 'Route Block',
      'family.replication_policy': 'Replication Policy',
      'family.replication_status': 'Replication Status',
      'family.replication_generation': 'Replication Generation',
      'family.failover_policy': 'Failover Policy',
      'family.failover_event': 'Failover Event',
      'family.protection_group': 'Protection Group',
      'family.storage_test': 'Storage Connection Test',
      'family.proxy_fleet_config': 'Proxy Fleet Config',
      'action.create': 'Create',
      'action.update': 'Update',
      'action.delete': 'Delete',
      'action.set': 'Set',
      'action.block': 'Block',
      'action.unblock': 'Unblock',
      'action.generation': 'Generation',
      'action.archive': 'Archive',
      'action.rotate': 'Rotate',
    },
    vi: {
      title: 'Nhật ký kiểm toán Control-Plane',
      pageDescription:
        'Lịch sử giao dịch các thay đổi cấu hình control-plane trên kho lưu trữ, danh tính S3, định tuyến, sao chép, chuyển đổi dự phòng và nhóm bảo vệ.',
      refresh: 'Làm mới',
      retry: 'Thử lại',
      applyFilters: 'Lọc kết quả',
      resetFilters: 'Đặt lại',
      errorTitle: 'Không thể tải nhật ký kiểm toán',
      errorText:
        'Control plane không phản hồi hoặc tính năng truy vấn nhật ký kiểm toán hiện không khả dụng.',
      backendLabel: 'Control-Plane Backend',
      backendPostgres: 'PostgreSQL (giao dịch bền vững)',
      backendRedis: 'Redis (tạm thời / không lưu nhật ký bền vững)',
      modeLabel: 'Chế độ ghi',
      modeReadOnly: 'Đang bật chế độ chỉ đọc (Read-only)',
      modeReadWrite: 'Đọc & ghi (Read-write)',
      redisBannerTitle:
        'Nhật ký kiểm toán giao dịch yêu cầu PostgreSQL backend',
      redisBannerText:
        'Control-plane hiện đang chạy trên Redis. Nhật ký kiểm toán giao dịch bền vững với bản chụp trước/sau (before/after JSON) sẽ hoạt động khi đặt controlPlane.backend thành "postgres".',
      readOnlyBannerTitle: 'Control plane đang ở chế độ chỉ đọc',
      readOnlyBannerText:
        'Các thao tác thay đổi cấu hình hiện đang bị khóa (controlPlane.readOnly đang bật). Lịch sử kiểm toán vẫn có thể truy vấn bình thường.',
      filtersTitle: 'Bộ lọc nhật ký kiểm toán',
      familyLabel: 'Nhóm đối tượng (Family)',
      familyAll: 'Tất cả nhóm',
      entityKeyLabel: 'Mã đối tượng (Entity Key)',
      entityKeyPlaceholder: 'VD: main, alice, pg-prod…',
      actorLabel: 'Tác nhân (Actor)',
      actorPlaceholder: 'VD: admin, system…',
      actionLabel: 'Hành động (Action)',
      actionAll: 'Tất cả hành động',
      limitLabel: 'Số dòng / trang',
      tableTitle: 'Danh sách bản ghi kiểm toán',
      tableMeta: 'Hiển thị {count} bản ghi',
      tableMetaCursor: '{count} bản ghi · trước #{cursor}',
      emptyTitle: 'Không có bản ghi kiểm toán nào',
      emptyText:
        'Không có thay đổi control-plane nào khớp với bộ lọc hiện tại hoặc chưa có thay đổi nào được ghi nhận.',
      colId: 'ID',
      colTimestamp: 'Thời gian',
      colActor: 'Tác nhân / Request',
      colAction: 'Hành động',
      colFamily: 'Nhóm (Family)',
      colEntityKey: 'Đối tượng (Entity)',
      colDetails: 'Chi tiết / So sánh',
      requestIdPrefix: 'req:',
      showDiff: 'Xem thay đổi',
      hideDiff: 'Ẩn thay đổi',
      openModal: 'Mở cửa sổ lớn',
      beforeSnapshot: 'Trước khi thay đổi (beforeJson)',
      afterSnapshot: 'Sau khi thay đổi (afterJson)',
      noBeforeSnapshot: 'Không có trạng thái trước (tạo mới hoặc khởi tạo)',
      noAfterSnapshot: 'Không có trạng thái sau (đã xóa)',
      redactedNote:
        'Khóa bí mật và thông tin nhạy cảm đã được che tự động ([REDACTED] / ***) trước khi lưu.',
      changedFields: 'Các trường thay đổi',
      noChangedFields: 'Không phát hiện khác biệt ở cấp trường gốc',
      paginationPage: 'Trang {page}',
      paginationFirst: 'Mới nhất',
      paginationNewer: '← Mới hơn',
      paginationOlder: 'Tải cũ hơn →',
      modalTitle: 'Bản ghi kiểm toán #{id} · {family}/{entityKey}',
      close: 'Đóng',
      langSwitchVi: 'Tiếng Việt',
      langSwitchEn: 'English',
      'family.storage': 'Kho lưu trữ (Storage)',
      'family.s3_identity': 'Danh tính S3',
      'family.dynamic_credential': 'Thông tin xác thực động',
      'family.proxy_account': 'Tài khoản Proxy',
      'family.access_policy': 'Chính sách truy cập',
      'family.console_user': 'Người dùng Console',
      'family.route': 'Chính sách định tuyến',
      'family.route_block': 'Chặn định tuyến',
      'family.replication_policy': 'Chính sách sao chép',
      'family.replication_status': 'Trạng thái sao chép',
      'family.replication_generation': 'Thế hệ sao chép',
      'family.failover_policy': 'Chính sách Failover',
      'family.failover_event': 'Sự kiện Failover',
      'family.protection_group': 'Nhóm bảo vệ',
      'family.storage_test': 'Kiểm tra kết nối kho',
      'family.proxy_fleet_config': 'Cấu hình cụm Proxy',
      'action.create': 'Tạo mới',
      'action.update': 'Cập nhật',
      'action.delete': 'Xóa',
      'action.set': 'Thiết lập',
      'action.block': 'Chặn',
      'action.unblock': 'Bỏ chặn',
      'action.generation': 'Tăng thế hệ',
      'action.archive': 'Lưu trữ',
      'action.rotate': 'Xoay vòng khóa',
    },
    [I18nLocale.DE]: {
      title: 'Control-Plane-Audit-Protokoll',
      pageDescription:
        'Transaktionsverlauf aller Konfigurationsänderungen der Control Plane über Speicher, Identitäten, Routing, Replikation, Failover und Schutzgruppen.',
      refresh: 'Aktualisieren',
      retry: 'Erneut versuchen',
      applyFilters: 'Filter anwenden',
      resetFilters: 'Zurücksetzen',
      errorTitle: 'Audit-Protokoll konnte nicht geladen werden',
      errorText:
        'Die Control Plane antwortet nicht oder das Audit-Protokoll ist nicht verfügbar.',
      backendLabel: 'Control-Plane-Backend',
      backendPostgres: 'PostgreSQL (transaktional)',
      backendRedis: 'Redis (flüchtig / ohne dauerhaftes Audit)',
      modeLabel: 'Schreibmodus',
      modeReadOnly: 'Schreibgeschützter Modus aktiv',
      modeReadWrite: 'Lesen & Schreiben',
      redisBannerTitle:
        'Transaktionales Audit-Protokoll erfordert PostgreSQL-Backend',
      redisBannerText:
        'Der Control-Plane-Status wird derzeit aus Redis bereitgestellt. Dauerhafte transaktionale Audit-Protokollierung ist aktiv, wenn controlPlane.backend auf "postgres" gesetzt ist.',
      readOnlyBannerTitle: 'Control Plane ist im schreibgeschützten Modus',
      readOnlyBannerText:
        'Konfigurationsänderungen sind derzeit gesperrt (controlPlane.readOnly ist aktiviert).',
      filtersTitle: 'Audit-Einträge filtern',
      familyLabel: 'Familie',
      familyAll: 'Alle Familien',
      entityKeyLabel: 'Objekt- / Entitäts-ID',
      entityKeyPlaceholder: 'z. B. main, alice, pg-prod…',
      actorLabel: 'Akteur',
      actorPlaceholder: 'z. B. admin, system…',
      actionLabel: 'Aktion',
      actionAll: 'Alle Aktionen',
      limitLabel: 'Seitengröße',
      tableTitle: 'Audit-Einträge',
      tableMeta: '{count} Einträge angezeigt',
      tableMetaCursor: '{count} Einträge · vor #{cursor}',
      emptyTitle: 'Keine Audit-Einträge gefunden',
      emptyText:
        'Keine Control-Plane-Änderungen entsprechen den aktuellen Filtern.',
      colId: 'ID',
      colTimestamp: 'Zeitstempel',
      colActor: 'Akteur / Anfrage',
      colAction: 'Aktion',
      colFamily: 'Familie',
      colEntityKey: 'Objekt-ID',
      colDetails: 'Details / Diff',
      requestIdPrefix: 'req:',
      showDiff: 'Diff anzeigen',
      hideDiff: 'Diff ausblenden',
      openModal: 'Modal öffnen',
      beforeSnapshot: 'Vorher (beforeJson)',
      afterSnapshot: 'Nachher (afterJson)',
      noBeforeSnapshot: 'Kein vorheriger Zustand (erstellt)',
      noAfterSnapshot: 'Kein Zielzustand (gelöscht)',
      redactedNote:
        'Geheimnisse und Zugangsdaten werden vor dem Speichern automatisch maskiert ([REDACTED] / ***).',
      changedFields: 'Geänderte Felder',
      noChangedFields: 'Keine Feldänderungen auf oberster Ebene erkannt',
      paginationPage: 'Seite {page}',
      paginationFirst: 'Neueste',
      paginationNewer: '← Neuer',
      paginationOlder: 'Ältere laden →',
      modalTitle: 'Audit-Eintrag #{id} · {family}/{entityKey}',
      close: 'Schließen',
      langSwitchVi: 'Tiếng Việt',
      langSwitchEn: 'English',
      'family.storage': 'Speicher',
      'family.s3_identity': 'S3-Identität',
      'family.dynamic_credential': 'Dynamische Zugangsdaten',
      'family.proxy_account': 'Proxy-Konto',
      'family.access_policy': 'Zugriffsrichtlinie',
      'family.console_user': 'Konsolenbenutzer',
      'family.route': 'Routing-Richtlinie',
      'family.route_block': 'Routensperre',
      'family.replication_policy': 'Replikationsrichtlinie',
      'family.replication_status': 'Replikationsstatus',
      'family.replication_generation': 'Replikationsgeneration',
      'family.failover_policy': 'Failover-Richtlinie',
      'family.failover_event': 'Failover-Ereignis',
      'family.protection_group': 'Schutzgruppe',
      'family.storage_test': 'Speichertest',
      'family.proxy_fleet_config': 'Proxy-Flottenkonfiguration',
      'action.create': 'Erstellen',
      'action.update': 'Aktualisieren',
      'action.delete': 'Löschen',
      'action.set': 'Setzen',
      'action.block': 'Sperren',
      'action.unblock': 'Entsperren',
      'action.generation': 'Generation',
      'action.archive': 'Archivieren',
      'action.rotate': 'Rotieren',
    },
  };

  const { t, locale } = useI18n({
    useScope: 'local',
    messages: auditMessages,
  });

  const isLoading = ref(false);
  const hasError = ref(false);
  const backend = ref<string>('postgres');
  const readOnly = ref<boolean>(false);
  const entries = ref<AuditLogEntry[]>([]);

  // Filter state
  const filterFamily = ref<string>('');
  const filterEntityKey = ref<string>('');
  const filterActor = ref<string>('');
  const filterAction = ref<string>('');
  const filterLimit = ref<number>(50);

  // Cursor pagination state
  const currentBeforeId = ref<string | number | undefined>(undefined);
  const cursorStack = ref<(string | number | undefined)[]>([]);

  // Expanded row & modal state
  const expandedIds = ref<Set<string>>(new Set());
  const modalEntry = ref<AuditLogEntry | null>(null);

  const familyOptions = computed(() => [
    { label: t('familyAll'), value: '' },
    ...AUDIT_LOG_FAMILIES.map((fam) => ({
      label: `${familyDisplayLabel(fam)} (${fam})`,
      value: fam,
    })),
  ]);

  const actionOptions = computed(() => [
    { label: t('actionAll'), value: '' },
    ...AUDIT_LOG_ACTIONS.map((act) => ({
      label: `${actionDisplayLabel(act)} (${act})`,
      value: act,
    })),
  ]);

  const limitOptions = [
    { label: '25', value: 25 },
    { label: '50', value: 50 },
    { label: '100', value: 100 },
    { label: '200', value: 200 },
  ];

  const hasActiveFilters = computed(
    () =>
      Boolean(filterFamily.value) ||
      Boolean(filterEntityKey.value.trim()) ||
      Boolean(filterActor.value.trim()) ||
      Boolean(filterAction.value.trim()) ||
      currentBeforeId.value !== undefined,
  );

  const currentPageNumber = computed(() => cursorStack.value.length + 1);

  const canLoadOlder = computed(() => {
    if (isLoading.value || entries.value.length < filterLimit.value) {
      return false;
    }

    const last = entries.value[entries.value.length - 1];

    return Boolean(last && last.id !== undefined && last.id !== '');
  });

  const canLoadNewer = computed(
    () => !isLoading.value && cursorStack.value.length > 0,
  );

  const tableMetaText = computed(() =>
    currentBeforeId.value !== undefined
      ? t('tableMetaCursor', {
          count: entries.value.length,
          cursor: currentBeforeId.value,
        })
      : t('tableMeta', { count: entries.value.length }),
  );

  function toggleBilingualLocale() {
    locale.value = locale.value === 'vi' ? I18nLocale.EN : 'vi';
  }

  function familyDisplayLabel(family: string): string {
    if (!family) return EMPTY_VALUE;

    const directKey = `family.${family}`;
    const directTranslated = t(directKey);

    if (directTranslated !== directKey) return directTranslated;

    const singular = family.endsWith('ies')
      ? family.slice(0, -3) + 'y'
      : family.endsWith('s')
        ? family.slice(0, -1)
        : family;
    const singKey = `family.${singular}`;
    const singTranslated = t(singKey);

    if (singTranslated !== singKey) return singTranslated;

    return family;
  }

  function actionDisplayLabel(action: string): string {
    if (!action) return EMPTY_VALUE;

    const key = `action.${action.toLowerCase()}`;
    const translated = t(key);

    return translated !== key ? translated : action;
  }

  function actionVariant(action: string): StatusVariant {
    const normalized = (action || '').trim().toLowerCase();

    switch (normalized) {
      case 'create':

      case 'unblock':
        return 'success';

      case 'update':

      case 'set':

      case 'generation':
        return 'info';

      case 'delete':

      case 'block':
        return 'danger';

      case 'rotate':

      case 'archive':
        return 'warning';

      default:
        return 'neutral';
    }
  }

  async function fetchAuditLog() {
    isLoading.value = true;
    hasError.value = false;

    try {
      const response = await AuditLogService.list({
        family: filterFamily.value || undefined,
        entityKey: filterEntityKey.value.trim() || undefined,
        actor: filterActor.value.trim() || undefined,
        action: filterAction.value.trim() || undefined,
        beforeId: currentBeforeId.value,
        limit: filterLimit.value,
      });

      backend.value = response.backend || 'redis';
      readOnly.value = Boolean(response.readOnly);
      entries.value = response.entries ?? [];
      expandedIds.value = new Set();
    } catch {
      hasError.value = true;
    } finally {
      isLoading.value = false;
    }
  }

  function applyFilters() {
    cursorStack.value = [];
    currentBeforeId.value = undefined;
    fetchAuditLog();
  }

  function resetFilters() {
    filterFamily.value = '';
    filterEntityKey.value = '';
    filterActor.value = '';
    filterAction.value = '';
    filterLimit.value = 50;
    cursorStack.value = [];
    currentBeforeId.value = undefined;
    fetchAuditLog();
  }

  function loadOlder() {
    if (!canLoadOlder.value) return;

    const last = entries.value[entries.value.length - 1];

    if (!last || last.id === undefined || last.id === '') return;

    cursorStack.value = [...cursorStack.value, currentBeforeId.value];
    currentBeforeId.value = last.id;
    fetchAuditLog();
  }

  function loadNewer() {
    if (!canLoadNewer.value) return;

    const nextStack = [...cursorStack.value];
    const previousCursor = nextStack.pop();

    cursorStack.value = nextStack;
    currentBeforeId.value = previousCursor;
    fetchAuditLog();
  }

  function loadLatest() {
    if (cursorStack.value.length === 0 && currentBeforeId.value === undefined) {
      return;
    }

    cursorStack.value = [];
    currentBeforeId.value = undefined;
    fetchAuditLog();
  }

  function rowKey(entry: AuditLogEntry): string {
    return String(entry.id);
  }

  function isExpanded(entry: AuditLogEntry): boolean {
    return expandedIds.value.has(rowKey(entry));
  }

  function toggleExpand(entry: AuditLogEntry) {
    const next = new Set(expandedIds.value);
    const key = rowKey(entry);

    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }

    expandedIds.value = next;
  }

  function formatJsonPayload(raw?: string): string {
    if (!raw || !raw.trim()) {
      return '';
    }

    try {
      const parsed = JSON.parse(raw);

      return JSON.stringify(parsed, null, 2);
    } catch {
      return raw;
    }
  }

  function parseJsonObject(raw?: string): Record<string, unknown> | null {
    if (!raw || !raw.trim()) return null;

    try {
      const parsed = JSON.parse(raw);

      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        return parsed as Record<string, unknown>;
      }

      return null;
    } catch {
      return null;
    }
  }

  function changedTopLevelKeys(entry: AuditLogEntry): string[] {
    const beforeObj = parseJsonObject(entry.beforeJson);
    const afterObj = parseJsonObject(entry.afterJson);

    if (!beforeObj && !afterObj) return [];

    const keys = new Set<string>([
      ...Object.keys(beforeObj ?? {}),
      ...Object.keys(afterObj ?? {}),
    ]);
    const changed: string[] = [];

    keys.forEach((key) => {
      const beforeVal = JSON.stringify(beforeObj?.[key]);
      const afterVal = JSON.stringify(afterObj?.[key]);

      if (beforeVal !== afterVal) {
        changed.push(key);
      }
    });

    return changed;
  }

  onBeforeMount(() => {
    fetchAuditLog();
  });
</script>

<template>
  <div class="tutti-audit-log-page">
    <PageHeader
      :title="t('title')"
      :description="t('pageDescription')"
    >
      <template #actions>
        <CButton
          size="small"
          @click="toggleBilingualLocale"
        >
          {{ locale === 'vi' ? t('langSwitchEn') : t('langSwitchVi') }}
        </CButton>
        <CButton
          :loading="isLoading"
          @click="fetchAuditLog()"
        >
          <template #icon>
            <CIcon
              :is-inline="true"
              :name="IconName.BASE_REFRESH"
            />
          </template>
          {{ t('refresh') }}
        </CButton>
      </template>

      <template #extra>
        <div class="tutti-audit-log-page__status-bar">
          <div class="tutti-audit-log-page__status-item">
            <span class="tutti-audit-log-page__status-label">
              {{ t('backendLabel') }}:
            </span>
            <StatusTag
              :variant="backend === 'postgres' ? 'success' : 'info'"
              :label="
                backend === 'postgres'
                  ? t('backendPostgres')
                  : t('backendRedis')
              "
            />
          </div>

          <div class="tutti-audit-log-page__status-item">
            <span class="tutti-audit-log-page__status-label">
              {{ t('modeLabel') }}:
            </span>
            <StatusTag
              :variant="readOnly ? 'warning' : 'success'"
              :label="readOnly ? t('modeReadOnly') : t('modeReadWrite')"
            />
          </div>
        </div>
      </template>
    </PageHeader>

    <ErrorState
      v-if="hasError"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-label="t('retry')"
      @retry="fetchAuditLog()"
    />

    <div
      v-else
      class="tutti-audit-log-page__content"
    >
      <Callout
        v-if="backend === 'redis'"
        tone="info"
        :title="t('redisBannerTitle')"
      >
        {{ t('redisBannerText') }}
      </Callout>

      <Callout
        v-if="readOnly"
        tone="warning"
        :title="t('readOnlyBannerTitle')"
      >
        {{ t('readOnlyBannerText') }}
      </Callout>

      <SectionCard :title="t('filtersTitle')">
        <form
          class="tutti-audit-log-page__filters"
          @submit.prevent="applyFilters"
        >
          <label class="tutti-audit-log-page__field">
            <span class="tutti-audit-log-page__field-label">{{
              t('familyLabel')
            }}</span>
            <NSelect
              v-model:value="filterFamily"
              :options="familyOptions"
              filterable
              clearable
              :disabled="isLoading"
              @update:value="applyFilters"
            />
          </label>

          <label class="tutti-audit-log-page__field">
            <span class="tutti-audit-log-page__field-label">{{
              t('entityKeyLabel')
            }}</span>
            <NInput
              v-model:value="filterEntityKey"
              :placeholder="t('entityKeyPlaceholder')"
              clearable
              :disabled="isLoading"
              @keyup.enter="applyFilters"
            />
          </label>

          <label class="tutti-audit-log-page__field">
            <span class="tutti-audit-log-page__field-label">{{
              t('actorLabel')
            }}</span>
            <NInput
              v-model:value="filterActor"
              :placeholder="t('actorPlaceholder')"
              clearable
              :disabled="isLoading"
              @keyup.enter="applyFilters"
            />
          </label>

          <label class="tutti-audit-log-page__field">
            <span class="tutti-audit-log-page__field-label">{{
              t('actionLabel')
            }}</span>
            <NSelect
              v-model:value="filterAction"
              :options="actionOptions"
              filterable
              tag
              clearable
              :disabled="isLoading"
              @update:value="applyFilters"
            />
          </label>

          <label
            class="tutti-audit-log-page__field tutti-audit-log-page__field--small"
          >
            <span class="tutti-audit-log-page__field-label">{{
              t('limitLabel')
            }}</span>
            <NSelect
              v-model:value="filterLimit"
              :options="limitOptions"
              :disabled="isLoading"
              @update:value="applyFilters"
            />
          </label>

          <div class="tutti-audit-log-page__filter-actions">
            <CButton
              type="primary"
              :loading="isLoading"
              @click="applyFilters"
            >
              {{ t('applyFilters') }}
            </CButton>
            <CButton
              :disabled="isLoading || !hasActiveFilters"
              @click="resetFilters"
            >
              {{ t('resetFilters') }}
            </CButton>
          </div>
        </form>
      </SectionCard>

      <SectionCard
        :title="t('tableTitle')"
        :meta="tableMetaText"
        flush
      >
        <template #actions>
          <div class="tutti-audit-log-page__pagination">
            <span class="tutti-audit-log-page__page-badge">
              {{ t('paginationPage', { page: currentPageNumber }) }}
            </span>
            <CButton
              size="small"
              :disabled="!canLoadNewer"
              @click="loadLatest"
            >
              {{ t('paginationFirst') }}
            </CButton>
            <CButton
              size="small"
              :disabled="!canLoadNewer"
              @click="loadNewer"
            >
              {{ t('paginationNewer') }}
            </CButton>
            <CButton
              size="small"
              :disabled="!canLoadOlder"
              @click="loadOlder"
            >
              {{ t('paginationOlder') }}
            </CButton>
          </div>
        </template>

        <EmptyState
          v-if="!isLoading && entries.length === 0"
          :icon="IconName.BASE_DOCUMENT"
          :title="t('emptyTitle')"
          :text="t('emptyText')"
        >
          <CButton
            v-if="hasActiveFilters"
            @click="resetFilters"
          >
            {{ t('resetFilters') }}
          </CButton>
        </EmptyState>

        <div
          v-else
          class="tt-table-wrap"
        >
          <table class="tt-table tutti-audit-log-page__table">
            <thead>
              <tr>
                <th scope="col">{{ t('colId') }}</th>
                <th scope="col">{{ t('colTimestamp') }}</th>
                <th scope="col">{{ t('colActor') }}</th>
                <th scope="col">{{ t('colAction') }}</th>
                <th scope="col">{{ t('colFamily') }}</th>
                <th scope="col">{{ t('colEntityKey') }}</th>
                <th
                  scope="col"
                  class="tt-num"
                >
                  {{ t('colDetails') }}
                </th>
              </tr>
            </thead>
            <tbody>
              <template
                v-for="entry in entries"
                :key="rowKey(entry)"
              >
                <tr
                  class="tutti-audit-log-page__row"
                  :class="{
                    'tutti-audit-log-page__row--expanded': isExpanded(entry),
                  }"
                  @click="toggleExpand(entry)"
                >
                  <td class="tt-mono tutti-audit-log-page__id-cell">
                    #{{ entry.id }}
                  </td>
                  <td>
                    <NTooltip placement="top">
                      <template #trigger>
                        <div>
                          <div>{{ formatDateTime(entry.at) }}</div>
                          <small class="tutti-audit-log-page__sub">
                            {{ formatRelative(entry.at) }}
                          </small>
                        </div>
                      </template>
                      {{ entry.at || EMPTY_VALUE }}
                    </NTooltip>
                  </td>
                  <td>
                    <div class="tutti-audit-log-page__actor">
                      <strong>{{ entry.actor || EMPTY_VALUE }}</strong>
                      <span
                        v-if="entry.requestId"
                        class="tutti-audit-log-page__req-badge tt-mono"
                      >
                        {{ t('requestIdPrefix') }} {{ entry.requestId }}
                      </span>
                    </div>
                  </td>
                  <td>
                    <StatusTag
                      :variant="actionVariant(entry.action)"
                      :label="actionDisplayLabel(entry.action)"
                    />
                  </td>
                  <td>
                    <div class="tutti-audit-log-page__family">
                      <span>{{ familyDisplayLabel(entry.family) }}</span>
                      <code class="tutti-audit-log-page__code-chip tt-mono">
                        {{ entry.family || EMPTY_VALUE }}
                      </code>
                    </div>
                  </td>
                  <td class="tt-mono">
                    <strong>{{ entry.entityKey || EMPTY_VALUE }}</strong>
                  </td>
                  <td
                    class="tt-num"
                    @click.stop
                  >
                    <div class="tutti-audit-log-page__row-actions">
                      <CButton
                        size="small"
                        @click="toggleExpand(entry)"
                      >
                        {{ isExpanded(entry) ? t('hideDiff') : t('showDiff') }}
                      </CButton>
                      <CButton
                        size="small"
                        @click="modalEntry = entry"
                      >
                        <template #icon>
                          <CIcon
                            :is-inline="true"
                            :name="IconName.BASE_EYE"
                          />
                        </template>
                      </CButton>
                    </div>
                  </td>
                </tr>

                <tr
                  v-if="isExpanded(entry)"
                  class="tutti-audit-log-page__diff-row"
                >
                  <td colspan="7">
                    <div class="tutti-audit-log-page__diff-container">
                      <div class="tutti-audit-log-page__diff-meta">
                        <div class="tutti-audit-log-page__diff-fields">
                          <span class="tutti-audit-log-page__diff-label">
                            {{ t('changedFields') }}:
                          </span>
                          <template v-if="changedTopLevelKeys(entry).length">
                            <code
                              v-for="field in changedTopLevelKeys(entry)"
                              :key="field"
                              class="tutti-audit-log-page__code-chip tt-mono"
                            >
                              {{ field }}
                            </code>
                          </template>
                          <span
                            v-else
                            class="tutti-audit-log-page__sub"
                          >
                            {{ t('noChangedFields') }}
                          </span>
                        </div>
                        <span class="tutti-audit-log-page__redacted-note">
                          {{ t('redactedNote') }}
                        </span>
                      </div>

                      <div class="tutti-audit-log-page__diff-grid">
                        <div class="tutti-audit-log-page__diff-pane">
                          <header class="tutti-audit-log-page__diff-header">
                            <strong>{{ t('beforeSnapshot') }}</strong>
                          </header>
                          <pre
                            v-if="formatJsonPayload(entry.beforeJson)"
                            class="tutti-audit-log-page__json tt-mono"
                            >{{ formatJsonPayload(entry.beforeJson) }}</pre
                          >
                          <div
                            v-else
                            class="tutti-audit-log-page__json-empty"
                          >
                            {{ t('noBeforeSnapshot') }}
                          </div>
                        </div>

                        <div class="tutti-audit-log-page__diff-pane">
                          <header class="tutti-audit-log-page__diff-header">
                            <strong>{{ t('afterSnapshot') }}</strong>
                          </header>
                          <pre
                            v-if="formatJsonPayload(entry.afterJson)"
                            class="tutti-audit-log-page__json tt-mono"
                            >{{ formatJsonPayload(entry.afterJson) }}</pre
                          >
                          <div
                            v-else
                            class="tutti-audit-log-page__json-empty"
                          >
                            {{ t('noAfterSnapshot') }}
                          </div>
                        </div>
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>

        <footer
          v-if="entries.length > 0"
          class="tutti-audit-log-page__footer"
        >
          <span class="tutti-audit-log-page__sub">{{ tableMetaText }}</span>
          <div class="tutti-audit-log-page__pagination">
            <span class="tutti-audit-log-page__page-badge">
              {{ t('paginationPage', { page: currentPageNumber }) }}
            </span>
            <CButton
              size="small"
              :disabled="!canLoadNewer"
              @click="loadLatest"
            >
              {{ t('paginationFirst') }}
            </CButton>
            <CButton
              size="small"
              :disabled="!canLoadNewer"
              @click="loadNewer"
            >
              {{ t('paginationNewer') }}
            </CButton>
            <CButton
              size="small"
              :disabled="!canLoadOlder"
              @click="loadOlder"
            >
              {{ t('paginationOlder') }}
            </CButton>
          </div>
        </footer>
      </SectionCard>
    </div>

    <NModal
      :show="modalEntry !== null"
      preset="card"
      :title="
        modalEntry
          ? t('modalTitle', {
              id: modalEntry.id,
              family: modalEntry.family,
              entityKey: modalEntry.entityKey || EMPTY_VALUE,
            })
          : ''
      "
      style="width: 960px; max-width: 96vw"
      @update:show="
        (val) => {
          if (!val) modalEntry = null;
        }
      "
    >
      <div
        v-if="modalEntry"
        class="tutti-audit-log-page__modal-body"
      >
        <div class="tutti-audit-log-page__modal-summary">
          <div>
            <span class="tutti-audit-log-page__status-label"
              >{{ t('colTimestamp') }}:</span
            >
            <strong>{{ formatDateTime(modalEntry.at) }}</strong>
            <span class="tutti-audit-log-page__sub tt-mono"
              >({{ modalEntry.at }})</span
            >
          </div>
          <div>
            <span class="tutti-audit-log-page__status-label"
              >{{ t('colActor') }}:</span
            >
            <strong>{{ modalEntry.actor || EMPTY_VALUE }}</strong>
            <code
              v-if="modalEntry.requestId"
              class="tutti-audit-log-page__code-chip tt-mono"
            >
              {{ t('requestIdPrefix') }} {{ modalEntry.requestId }}
            </code>
          </div>
          <div>
            <span class="tutti-audit-log-page__status-label"
              >{{ t('colAction') }}:</span
            >
            <StatusTag
              :variant="actionVariant(modalEntry.action)"
              :label="actionDisplayLabel(modalEntry.action)"
            />
          </div>
        </div>

        <p class="tutti-audit-log-page__redacted-note">
          {{ t('redactedNote') }}
        </p>

        <div class="tutti-audit-log-page__diff-grid">
          <div class="tutti-audit-log-page__diff-pane">
            <header class="tutti-audit-log-page__diff-header">
              <strong>{{ t('beforeSnapshot') }}</strong>
            </header>
            <pre
              v-if="formatJsonPayload(modalEntry.beforeJson)"
              class="tutti-audit-log-page__json tt-mono"
              >{{ formatJsonPayload(modalEntry.beforeJson) }}</pre
            >
            <div
              v-else
              class="tutti-audit-log-page__json-empty"
            >
              {{ t('noBeforeSnapshot') }}
            </div>
          </div>

          <div class="tutti-audit-log-page__diff-pane">
            <header class="tutti-audit-log-page__diff-header">
              <strong>{{ t('afterSnapshot') }}</strong>
            </header>
            <pre
              v-if="formatJsonPayload(modalEntry.afterJson)"
              class="tutti-audit-log-page__json tt-mono"
              >{{ formatJsonPayload(modalEntry.afterJson) }}</pre
            >
            <div
              v-else
              class="tutti-audit-log-page__json-empty"
            >
              {{ t('noAfterSnapshot') }}
            </div>
          </div>
        </div>
      </div>
    </NModal>
  </div>
</template>

<style lang="scss" scoped>
  .tutti-audit-log-page {
    &__content {
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    &__status-bar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 12px 24px;
      padding: 10px 14px;
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
    }

    &__status-item {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;
    }

    &__status-label {
      color: var(--tt-text-2);
      font-weight: 500;
    }

    &__filters {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
      gap: 14px;
      align-items: end;
    }

    &__field {
      display: flex;
      flex-direction: column;
      gap: 6px;
      min-width: 0;

      &--small {
        max-width: 140px;
      }
    }

    &__field-label {
      font-size: 12px;
      font-weight: 600;
      color: var(--tt-text-2);
    }

    &__filter-actions {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    &__pagination {
      display: inline-flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 6px;
    }

    &__page-badge {
      font-size: 12px;
      font-weight: 600;
      color: var(--tt-text-2);
      margin-right: 4px;
    }

    &__row {
      cursor: pointer;

      &:hover {
        background-color: var(--tt-hover);
      }

      &--expanded {
        background-color: var(--tt-accent-subtle);
      }
    }

    &__id-cell {
      font-weight: 600;
      color: var(--tt-text-2);
    }

    &__sub {
      display: block;
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__actor {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    &__req-badge {
      display: inline-block;
      font-size: 11px;
      color: var(--tt-text-3);
    }

    &__family {
      display: flex;
      flex-direction: column;
      gap: 2px;
      align-items: flex-start;
    }

    &__code-chip {
      display: inline-block;
      padding: 1px 6px;
      font-size: 11px;
      border-radius: var(--tt-radius-tag);
      background-color: var(--tt-code-bg);
      color: var(--tt-text-2);
      border: 1px solid var(--tt-border);
    }

    &__row-actions {
      display: inline-flex;
      align-items: center;
      justify-content: flex-end;
      gap: 6px;
    }

    &__diff-row td {
      padding: 0;
      background-color: var(--tt-subtle);
      white-space: normal;
    }

    &__diff-container {
      padding: 16px;
      border-top: 1px solid var(--tt-border);
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    &__diff-meta {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: 8px 16px;
    }

    &__diff-fields {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 6px;
    }

    &__diff-label {
      font-size: 12px;
      font-weight: 600;
      color: var(--tt-text-2);
    }

    &__redacted-note {
      margin: 0;
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__diff-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 14px;
    }

    &__diff-pane {
      display: flex;
      flex-direction: column;
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      overflow: hidden;
      min-width: 0;
    }

    &__diff-header {
      padding: 8px 12px;
      font-size: 12px;
      background-color: var(--tt-subtle);
      border-bottom: 1px solid var(--tt-border);
      color: var(--tt-text-2);
    }

    &__json {
      margin: 0;
      padding: 12px;
      font-size: 12px;
      line-height: 1.5;
      color: var(--tt-text);
      overflow-x: auto;
      max-height: 360px;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }

    &__json-empty {
      padding: 20px 12px;
      font-size: 13px;
      color: var(--tt-text-3);
      font-style: italic;
    }

    &__footer {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 12px 16px;
      border-top: 1px solid var(--tt-border);
      background-color: var(--tt-surface);
    }

    &__modal-body {
      display: flex;
      flex-direction: column;
      gap: 14px;
    }

    &__modal-summary {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 12px 24px;
      padding: 10px 12px;
      background-color: var(--tt-subtle);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-control);
      font-size: 13px;

      > div {
        display: inline-flex;
        align-items: center;
        gap: 6px;
      }
    }
  }
</style>
