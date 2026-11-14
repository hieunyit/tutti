# Thiết kế lại giao diện Tutti Console (UI redesign)

Trạng thái: chờ duyệt. Phạm vi: chỉ `ui/` (Vue 3, Naive UI, Pinia, vue-i18n). Backend và API giữ nguyên.

## 1. Mục tiêu và ràng buộc

Đã thống nhất với người dùng:
- Làm lại cả hình thức lẫn cấu trúc menu/luồng dùng.
- Hỗ trợ theme sáng và tối, mặc định theo hệ điều hành (đã có `colorSchemeStore`).
- Giữ Naive UI; đổi theme + viết thêm component dùng chung. Không thay thư viện.
- Menu theo phương án B: 5 nhóm, 10 mục (mục 2).
- Mỗi page được thiết kế chi tiết (mục 4) và giao cho agent riêng (mục 6).

Tiêu chí thành công:
1. Menu 15 mục giảm còn 10; URL cũ vẫn dùng được qua redirect.
2. Mọi page có đủ 4 trạng thái: loading (skeleton), empty, error (có nút thử lại), content.
3. Không page nào quá 400 dòng; logic tách ra store/composable/component con.
4. Sáng và tối đều đạt độ tương phản WCAG AA; dùng được bằng bàn phím; chạy tốt từ 375px đến 1280px.
5. `vue-tsc`, `vitest`, `eslint`, `vite build` đều pass.

Không làm: đổi backend/API, đổi tên route name (chỉ thêm), thêm tính năng mới ngoài danh sách dưới.

## 2. Cấu trúc menu (phương án B)

| Nhóm | Mục | Route mới | Ghi chú |
|---|---|---|---|
| Tổng quan | Dashboard | `/dashboard` | Gộp Home + Monitoring; tab `overview`, `metrics` (query `?tab=`) |
| Lưu trữ | Storages | `/storages`, `/storages/:name` | Giữ |
| Lưu trữ | Proxies | `/proxies` | Tab `fleet`, `config`; gộp Health vào đây |
| Bảo vệ dữ liệu | Protection groups | `/protection-groups`, `/:name` | Giữ |
| Bảo vệ dữ liệu | Replication | `/replication` | Tab `replications`, `routing`, `diff` |
| Truy cập | S3 users | `/users` | Đổi nhãn từ "Users" |
| Truy cập | Proxy accounts | `/proxy-accounts` | Giữ |
| Truy cập | Console users | `/console-users` | Admin |
| Quản trị | System settings | `/system-settings` | Admin |
| Quản trị | Audit log | `/audit-log` | Admin |

Redirect (giữ link cũ): `/home`, `/monitoring` → `/dashboard`; `/health` → `/proxies`;
`/routing-policies` → `/replication?tab=routing`; `/routing-policies/add` → `/replication/routing/add`;
`/diff-reports` → `/replication?tab=diff`; `/diff-reports/add|detail` → `/replication/diff/add|:id`; `/failover` → `/protection-groups` (đã có).

Thanh trên (topbar) thay cho nhóm Help trong menu: breadcrumb, tìm nhanh (Ctrl/Cmd+K), nút Trợ giúp (Docs, API reference), đổi theme sáng/tối/hệ thống, menu người dùng (đổi mật khẩu, đăng xuất).
Sidebar: thu gọn thành rail icon trên desktop, drawer trên mobile (đã có, giữ lại). Badge cảnh báo trên Storages, Proxies, Protection groups giữ nguyên (`useNavBadges`).

## 3. Design system

Nền tảng hiện có: `ui/src/styles/tokens.scss` (khoảng 129 biến `--tt-*`, ghi đè dark ở `body.color-scheme-dark`), `ui/src/styles/theme.ts` (Naive UI overrides sáng/tối), `App.vue` nạp theme.

Quy tắc:
- **Màu**: 60-30-10. 60% nền (`--tt-bg`, `--tt-surface`), 30% chữ + viền (`--tt-text`, `--tt-border`), 10% màu nhấn duy nhất (`--tt-accent`) cho hành động chính. Màu ngữ nghĩa: `success`, `warning`, `danger`, `info` chỉ dùng cho trạng thái, không trang trí. Không gradient, không glassmorphism.
- **Độ cao**: viền trước, bóng mềm sau; bóng chỉ cho popover/drawer/modal.
- **Chữ**: một họ font hệ thống + mono cho giá trị kỹ thuật (key, bucket, ID). Thang 12/13/14/16/20/24; tối đa 3 độ đậm (400/500/600).
- **Khoảng cách**: lưới 4/8px (`--tt-space-1..8`); chiều rộng nội dung tối đa 1280px; độ dài dòng văn bản ≤ 72ch.
- **Bo góc**: 6px control, 10px card, 14px modal. **Chuyển động**: 120/200ms, chỉ opacity/transform; tôn trọng `prefers-reduced-motion`.
- **Mật độ**: bảng mặc định "comfortable" 44px/dòng, tùy chọn "compact" 32px (lưu localStorage).

Bộ component dùng chung (đặt ở `ui/src/components/kit/`, do WP0 làm; các WP khác chỉ import):

| Component | Việc |
|---|---|
| `PageShell` | Khung page: tiêu đề, mô tả, vùng hành động chính (phải), tab (query-synced), nội dung; xử lý 4 trạng thái qua prop `state` |
| `DataTable` | Bọc `NDataTable`: cột, sắp xếp, phân trang, chọn nhiều, thanh hành động hàng loạt, skeleton/empty/error có sẵn |
| `FilterBar` | Ô tìm + bộ lọc dạng chip, nút "Xóa lọc" |
| `StatTile` | Số liệu KPI + xu hướng + trạng thái màu |
| `DetailLayout` | Trang chi tiết: header (tên, trạng thái, hành động) + tab + cột phụ |
| `FormSection` | Nhóm trường có nhãn, mô tả, lỗi inline (kiểm tra khi blur) |
| `WizardShell` | Wizard nhiều bước, giữ bản nháp khi chuyển trang, nút Next/Back cố định |
| `ConfirmDialog` | Hành động phá hủy: nêu tên đối tượng, nút đỏ, gõ tên để xác nhận với thao tác nặng |
| `StateBlock` | `LoadingBlock`, `EmptyBlock` (có CTA), `ErrorBlock` (có Thử lại) |
| `StatusBadge` | Chấm màu + chữ (không chỉ dựa vào màu) |
| `CopyValue` | Giá trị mono + nút sao chép; chế độ "hiện một lần" cho secret |

Tái dùng/di dời: `components/ui/*`, `components/tutti/common/*` (BackLink, TabBar, ErrorState, FilterBar…) được gộp vào kit, các file cũ giữ re-export để không vỡ import trong lúc chuyển.

## 4. Thiết kế từng page

Quy ước chung mỗi page: dùng `PageShell`; skeleton khi tải > 100ms; empty có CTA; error có "Thử lại"; hành động ghi bọc `WriteGate`; chuỗi mới thêm vào i18n en/de/vi.

### 4.1 Dashboard (`/dashboard`) — gộp TuttiHomePage + TuttiMonitoringPage
- Tab **Overview**: hàng `StatTile` (storages khỏe, proxies online, replications đang chạy, cảnh báo); khối "Cần chú ý" (storage lỗi, proxy cấu hình lỗi, replication tụt lag) tối đa 5 dòng có link; danh sách storage rút gọn; danh sách replication rút gọn; endpoint proxy + sao chép credential (CopyValue).
- Tab **Metrics**: chọn khoảng thời gian (1h/6h/24h/7d), biểu đồ traffic, breakdown theo storage/user, KPI replication. Uptime proxy/worker chỉ hiện khi `HAS_PROMETHEUS`.
- Nút làm mới + "cập nhật lúc hh:mm"; tự làm mới 30s khi tab đang hiển thị.
- Tái cấu trúc: tách `useDashboardData` (store hiện có `useTuttiDashboardStore` + `useTuttiMonitoringStore` giữ nguyên API, chỉ gọi từ một chỗ); mỗi widget tự có `StateBlock` nhỏ, thêm một error boundary cấp page.

### 4.2 Storages (`/storages`) — TuttiStoragesPage
- `StatTile`: available, requests, coverage, reporting. `FilterBar`: tên, health, managed. `DataTable` cột: tên (link), health (`StatusBadge`), breaker, requests, users, replication, managed, credentials.
- Hành động: "Thêm storage" (drawer `AddStorageDialog`, bọc `WriteGate`). Hàng bấm vào mở chi tiết.
- Empty: giải thích storage là gì + CTA thêm storage.

### 4.3 Storage details (`/storages/:name`) — TuttiStorageDetailsPage
- `DetailLayout`: header (tên, health, breaker, nút Làm mới). Tab (query-synced): `overview` (KPI requests/lag/users/rate/rpm, replication sourceOf/targetOf, failover, sự kiện), `health` (sức khỏe theo proxy), `users`, `settings`.
- Tab settings tách khỏi `StorageSettings` 468 dòng: mỗi nhóm trường là một `FormSection` + nút Lưu riêng, có cảnh báo thay đổi chưa lưu.

### 4.4 Proxies (`/proxies`) — TuttiProxiesPage (867 dòng) + TuttiHealthPage
- Tab **Fleet**: bảng proxy (uptime, version, status, pending restart, config error, sync) + thẻ sức khỏe từng proxy (nội dung của Health page). Tự làm mới 15s, có thể tạm dừng.
- Tab **Config**: editor cấu hình fleet chia 5 `FormSection` (Sync, Health, Writer lease/quorum, Timeouts, Storages) + per-group overrides; Lưu qua `ConfirmDialog` có tóm tắt thay đổi (diff trước/sau).
- Tái cấu trúc bắt buộc: tách store `useTuttiProxiesStore` và `useTuttiProxyFleetConfigStore` (page đang giữ state cục bộ + ~15 setter + interval); `TuttiHealthPage` chỉ còn redirect.

### 4.5 Protection groups (`/protection-groups`) — TuttiProtectionGroupsPage
- `StatTile` (số nhóm, đang switchover, lag tối đa). Công tắc "Tự động freeze" trong header với `ConfirmDialog`. `DataTable`: nhóm, scope, copies, max lag, thay đổi gần nhất, trạng thái. "Tạo nhóm" mở `ProtectionGroupDialog` (561 dòng) tách thành `WizardShell` 3 bước: Chọn scope → Chọn copies/primary → Xác nhận.
- Lỗi của freeze và lỗi danh sách hiển thị riêng nhưng cùng kiểu `ErrorBlock`.

### 4.6 Protection group detail (`/protection-groups/:name`) — 1101 dòng
- `DetailLayout`: header (tên, trạng thái, primary, hành động chính "Switchover"). Tab: `members` (thẻ từng copy, primary đánh dấu), `writers` (danh sách + Clear writer), `activity` (lịch sử switchover/abort), `settings` (sửa, xóa vùng nguy hiểm).
- Hành động nguy hiểm (failover ngay, abort, clear writer, xóa) đều qua `ConfirmDialog` nêu tên nhóm; "xóa" yêu cầu gõ tên.
- Tách thành 4 component con theo tab + `useProtectionGroupActions` composable; page ≤ 250 dòng.

### 4.7 Replication hub (`/replication`)
Một page, 3 tab query-synced; mỗi tab giữ store hiện có.
- Tab **Replications** (TuttiReplicationPage): `FilterBar` (status, direction, type, user, bucket, ngày tạo); `DataTable` cột bucket, direction, status, tiến độ, tạo lúc, hành động; chọn nhiều → Pause/Resume/Delete hàng loạt; "Thêm replication" → `/replication/add`.
- Tab **Routing** (TuttiRoutingPoliciesPage): bảng user/storage/bucket/blocked; Block/Unblock/Sửa/Xóa; "Thêm policy".
- Tab **Diff** (TuttiDiffReportsPage): bảng status/progress/direction/bucket; Restart/Xóa; "Tạo diff report"; hàng mở trang chi tiết.
- Wizard **Add replication** (`/replication/add`, TuttiAddReplicationPage): `WizardShell` 4 bước From → To → Buckets → Xác nhận; chọn bucket (313 dòng) dùng bảng có tìm kiếm + chọn tất cả; bản nháp được giữ, nút "Làm lại từ đầu".
- Wizard **Add routing policy** (`/replication/routing/add`): User → Storage → Bucket → Status → Xác nhận.
- Wizard **Add diff report** (`/replication/diff/add`): From → To → Bucket/User → Tùy chọn → Xác nhận.
- **Diff detail** (`/replication/diff/:id`): `DetailLayout` header (trạng thái, Restart), tab `overview` + tiến độ, tab `entries` (lọc, phân trang theo cursor).
- Dọn: gộp ba bộ Tile/ListActions/ActionsCell trùng lặp thành `DataTable` + `useRowActions`; thêm loading/error cho các host Add* và Diff detail.

### 4.8 S3 users (`/users`) — TuttiUsersPage + UserCreate + UserDetail
- **Danh sách**: `SegmentedFilter` (All / Incomplete / …, có đếm) + `DataTable` (tên, trạng thái, credentials, nhóm bảo vệ); empty phân biệt "chưa có user" và "lọc ra 0 kết quả".
- **Tạo user** (`/users/new`, 777 dòng): `WizardShell` 3 bước Danh tính → Quyền → Khóa; secret hiện một lần bằng `CopyValue` chế độ one-time kèm xác nhận "đã lưu". Tách logic thành `useUserCreate` + 3 component bước.
- **Chi tiết** (`/users/:name`, 670 dòng): `DetailLayout`; tab (query-synced) `credentials`, `keys`, `permissions`; hành động Test user, Xóa user (vùng nguy hiểm, `ConfirmDialog` gõ tên), Rotate key; `AccessPolicyDrawer` (800 dòng) tách theo nhóm quyền.

### 4.9 Proxy accounts (`/proxy-accounts`)
- `DataTable`: tên, trạng thái, policy, tạo lúc; hành động Rotate secret, Bật/Tắt, Xóa, Sửa policy. "Tạo account" mở modal; secret hiện một lần (`CopyValue`). Khi dynamic mode tắt: `EmptyBlock` giải thích cách bật.

### 4.10 Console users (`/console-users`, admin)
- `DataTable`: username, role, trạng thái, nguồn, lần đăng nhập cuối, đánh dấu "bạn". Hành động: Tạo (sinh mật khẩu một lần, `CopyValue` one-time), Đổi role, Đặt lại mật khẩu, Xóa; không cho xóa/hạ quyền chính mình. Banner khi auth bị tắt.

### 4.11 System settings (`/system-settings`, admin)
- `PageShell` với tab Features, Bulk, Worker, Locking, Webhook; mỗi tab các `FormSection`. Thanh dưới cố định khi có thay đổi: Lưu (mở tóm tắt diff), Hủy; cảnh báo rời trang khi chưa lưu; xử lý xung đột phiên bản bằng hộp thoại "Tải lại / Ghi đè". Trường chỉ áp dụng sau khi restart có nhãn "Cần restart worker".
- Tách `SystemSettingsHelper` + store giữ nguyên; page ≤ 250 dòng.

### 4.12 Audit log (`/audit-log`, admin) — 1426 dòng
- Tách thành: `AuditFilters` (action, family, actor, ngày, text), `AuditTable` (cursor paging Tải thêm cũ/mới), `AuditEntryDrawer` (diff JSON trước/sau), store mới `useAuditLogStore`, composable `useAuditCursor`. Banner read-only/Redis giữ nguyên. Page ≤ 200 dòng.

### 4.13 Login, đổi mật khẩu, Docs
- **Login** (655 dòng): màn hình đơn giản một cột, logo + form (username, password, hiện/ẩn mật khẩu), lỗi và số lần thử còn lại; trạng thái "không kết nối được server" có nút Thử lại; tách logic init/retry ra `useLoginFlow`. Sáng/tối theo hệ điều hành.
- **Change password**: dùng `FormSection`, hiển thị quy tắc mật khẩu, loading/error rõ; khi bắt buộc đổi thì ẩn điều hướng khác.
- **Docs** (`/docs`, `/docs/api`): mở từ menu Trợ giúp ở topbar; tab Guides và API reference, có skeleton cho swagger.

## 5. Quy tắc kỹ thuật cho mọi agent

- Chỉ sửa file trong phạm vi được giao (mục 6). Không sửa `routes.ts`, `navigation.ts`, `styles/`, `components/kit/` ngoài WP0.
- Import từ `@/components/kit`; không tự viết lại bảng, dialog xác nhận, trạng thái.
- Màu/khoảng cách/radius chỉ dùng biến `--tt-*`; không hard-code mã màu.
- Mọi chuỗi qua i18n (en, de, vi); nhãn nút dùng động từ ("Tạo storage", "Xóa user"), không dùng "OK/Submit".
- Mỗi page: có test `vitest` cho store/composable mới và một test render 4 trạng thái.
- Xong mỗi WP chạy: `yarn lint:types`, `yarn test:unit --run <phạm vi>`, `yarn lint`. Báo cáo ≤ 10 dòng.

## 6. Chia việc cho agent

Thứ tự: WP0 chạy trước (nền tảng, tuần tự). Sau khi WP0 xong, WP1–WP7 chạy song song vì không chung file. Cuối cùng WP8 tích hợp.

| WP | Phạm vi file (sở hữu) | Nội dung | Model gợi ý |
|---|---|---|---|
| WP0 Nền tảng | `styles/*`, `components/kit/*`, `components/layout/*`, `router/*`, `utils/constants/navigation.ts`, i18n menu | Token + theme sáng/tối, bộ kit, sidebar + topbar + Help menu, route mới + redirect | opus |
| WP1 Dashboard | `pages/TuttiHomePage`, `TuttiMonitoringPage` → `pages/DashboardPage`, `components/dashboard/*` | Mục 4.1 | sonnet |
| WP2 Storage + Proxies | `TuttiStoragesPage`, `TuttiStorageDetailsPage`, `TuttiProxiesPage`, `TuttiHealthPage`, `components/tutti/storages|proxies|health` | 4.2, 4.3, 4.4 | sonnet |
| WP3 Protection groups | `TuttiProtectionGroupsPage`, `TuttiProtectionGroupDetailPage`, `TuttiFailoverPage`, `components/tutti/protection*` | 4.5, 4.6 | sonnet |
| WP4 Replication hub | `TuttiReplicationPage`, `TuttiRoutingPoliciesPage`, `TuttiDiffReportsPage`, các trang `TuttiAdd*`, `TuttiDiffReportDetailPage`, `components/tutti/*` liên quan | 4.7 | sonnet (tách 4a replications + wizard, 4b routing + diff nếu quá dài) |
| WP5 Access | `TuttiUsersPage`, `TuttiUserCreatePage`, `TuttiUserDetailPage`, `TuttiProxyAccountsPage`, `TuttiConsoleUsersPage`, `components/tutti/users|access|proxy-accounts|console-users` | 4.8, 4.9, 4.10 | sonnet |
| WP6 Admin | `TuttiSystemSettingsPage`, `TuttiAuditLogPage`, `components/tutti/settings|audit` | 4.11, 4.12 | sonnet |
| WP7 Auth + Docs | `LoginPage`, `TuttiChangePasswordPage`, `DocsPage`, `components/auth/*`, `components/docs/*` | 4.13 | haiku/sonnet |
| WP8 Tích hợp | toàn `ui/` (chỉ đọc + sửa nhỏ) | `yarn build`, `yarn test:unit --run`, kiểm tra redirect, chụp ảnh sáng/tối từng page bằng trình duyệt, sửa lỗi tích hợp | sonnet |

Chi phí token: mỗi agent chỉ nhận mục 3, mục 5 và đúng mục của mình trong mục 4, không nhận toàn bộ spec.

## 7. Rủi ro và cách xử lý

- Redirect sai làm hỏng link cũ → WP8 kiểm tra từng route cũ.
- Hai WP cùng sửa một file dùng chung → quy tắc sở hữu file (mục 5); cần sửa kit thì báo lại WP0 hoặc WP8.
- Test hiện có bám vào cấu trúc cũ (vd `components/layout/__tests__`) → WP0 cập nhật test layout, mỗi WP cập nhật test trong phạm vi của mình.
- Page quá lớn khó tách → ưu tiên giữ API store, chỉ tách component và composable.
