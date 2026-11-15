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

export default {
  title: 'Nhật ký kiểm toán',
  retry: 'Thử lại',
  errorText: 'Không tải được nhật ký kiểm toán. Hãy thử lại.',
  loadOlder: 'Tải thêm bản ghi cũ hơn',
  loadNewer: 'Tải thêm bản ghi mới hơn',
  loadMoreFailed:
    'Không tải thêm được bản ghi. Các bản ghi đã tải vẫn được giữ lại.',
  allLoaded: 'Đã tải hết bản ghi',
  tableMeta: 'Hiển thị {count} bản ghi',
  emptyTitle: 'Chưa có bản ghi kiểm toán',
  noMatchTitle: 'Không có bản ghi nào khớp',
  clearFilters: 'Xóa bộ lọc',
  searchLabel: 'Tìm theo đối tượng',
  searchPlaceholder: 'VD: main, alice',
  familyLabel: 'Loại',
  familyAll: 'Tất cả loại',
  actionLabel: 'Hành động',
  actionAll: 'Tất cả hành động',
  actorLabel: 'Người thay đổi',
  actorPlaceholder: 'VD: admin',
  dateRangeLabel: 'Khoảng ngày',
  limitLabel: 'Số bản ghi mỗi lần tải',
  colTimestamp: 'Thời gian',
  colActor: 'Người thay đổi',
  colAction: 'Hành động',
  colObject: 'Đối tượng',
  requestId: 'Request ID',
  showDiff: 'Xem thay đổi',
  drawerTitle: 'Bản ghi #{id}',
  changedFields: 'Các trường thay đổi',
  noChangedFields: 'Không phát hiện thay đổi ở cấp trường',
  redactedNote: 'Thông tin bí mật được che trước khi lưu.',
  beforeSnapshot: 'Trước',
  afterSnapshot: 'Sau',
  noBeforeSnapshot: 'Không có trạng thái trước (tạo mới)',
  noAfterSnapshot: 'Không có trạng thái sau (đã xóa)',
  redisBannerTitle: 'Lịch sử không được lưu bền vững',
  redisBannerText:
    'Kho cấu hình đang là Redis, không ghi bản ghi kiểm toán kèm trạng thái trước và sau. Dùng PostgreSQL để giữ lịch sử bền vững.',
  readOnlyBannerTitle: 'Cấu hình đang ở chế độ chỉ đọc',
  readOnlyBannerText:
    'Thay đổi bị khóa khi control plane ở chế độ chỉ đọc. Lịch sử hiện có vẫn xem được.',
  family_storage: 'Storage',
  family_s3_identity: 'User',
  family_proxy_account: 'Access key',
  family_access_policy: 'Chính sách truy cập',
  family_dynamic_credential: 'Credential động',
  family_dynamic_credential_salt: 'Salt của credential',
  family_console_user: 'Người dùng console',
  family_console_user_login: 'Đăng nhập console',
  family_route: 'Routing rule',
  family_route_block: 'Route bị chặn',
  family_replication: 'Replication',
  family_replication_policy: 'Replication',
  family_replication_status: 'Trạng thái Replication',
  family_replication_generation: 'Thế hệ Replication',
  family_protection_group: 'Nhóm bảo vệ',
  family_failover_policy: 'Chính sách Failover',
  family_failover_event: 'Sự kiện Failover',
  family_diff: 'Báo cáo diff',
  family_diff_report: 'Báo cáo diff',
  family_storage_test: 'Kiểm tra kết nối Storage',
  family_proxy_fleet_config: 'Cài đặt Proxy',
  family_system_settings: 'Cài đặt hệ thống',
  action_create: 'Tạo',
  action_update: 'Cập nhật',
  action_delete: 'Xóa',
  action_set: 'Đặt',
  action_rename: 'Đổi tên',
  action_pause: 'Tạm dừng',
  action_resume: 'Tiếp tục',
  action_start: 'Bắt đầu',
  action_restart: 'Chạy lại',
  action_fix: 'Sửa',
  action_enable: 'Bật',
  action_disable: 'Tắt',
  action_block: 'Chặn',
  action_unblock: 'Bỏ chặn',
  action_generation: 'Tăng thế hệ',
  action_archive: 'Lưu trữ',
  action_rotate: 'Xoay vòng',
  action_clear_writer: 'Giải phóng writer',
  action_intent: 'Yêu cầu {action}',
};
