# Phiên đăng nhập trượt (gia hạn, hết hạn do rảnh, cảnh báo)

Trạng thái: chờ duyệt. Phạm vi: `pkg/auth` (backend), `ui/src/stores/authStore.ts` và lớp HTTP/giao diện đăng nhập, tài liệu.

## 1. Vì sao
Hiện phiên đăng nhập là token ngẫu nhiên đục lưu trong Redis với thời hạn **cố định 12 giờ kể từ lúc đăng nhập** (`pkg/auth/service.go:367`, `config.go:17`). Không có làm mới, nên người dùng đang làm việc vẫn bị đá ra đột ngột sau 12 giờ; giao diện không dùng `expiresAt` nên không cảnh báo trước và chỉ phản ứng khi gặp 401. Cũng không có hết hạn khi rảnh.

Người dùng chọn **cách 1: phiên trượt**, giữ token đục (không đổi sang JWT), và để tôi chọn các con số.

## 2. Quyết định
- Giữ token đục lưu phía server, thu hồi được ngay (đăng xuất, đổi mật khẩu, vô hiệu hóa user vẫn có hiệu lực như hiện tại).
- **Hết hạn do rảnh (idle): 30 phút.** **Trần tuyệt đối: 24 giờ** kể từ lúc đăng nhập, không gia hạn vượt quá. **Cảnh báo: 2 phút** trước khi hết hạn do rảnh, và 5 phút trước trần tuyệt đối.
- Đã chọn các con số này làm mặc định; cấu hình được.

## 3. Mục tiêu / không làm / tránh tuyệt đối
Mục tiêu: người đang làm việc không bị đá ra bất ngờ trong 24 giờ; phiên bị bỏ rảnh hết hạn sau 30 phút; người dùng luôn được cảnh báo trước và có nút "Tiếp tục đăng nhập".
Không làm: đổi sang JWT; refresh token trong cookie (cách 2); MFA; danh sách phiên đang mở.
Tuyệt đối tránh: để polling nền của giao diện (5 đến 30 giây một lần) tự gia hạn phiên, vì khi đó hết hạn do rảnh vô nghĩa; làm mất việc đang dở khi hết hạn mà không báo.

## 4. Thiết kế

### 4.1 Backend
- Phiên lưu thêm `LastActiveAt` và dùng sẵn `CreatedAt`. Thời hạn khóa Redis = `min(idle, trần − đã trôi qua)`. **`Authenticate` không gia hạn phiên** (để polling nền không giữ phiên sống).
- Thêm `POST /auth/refresh` (cần token hợp lệ, không cần `MustChangePassword` xử lý khác ngoài luật hiện có): đặt lại thời hạn khóa Redis về `min(idle, trần − đã trôi qua)` và cập nhật `LastActiveAt`; trả `{expiresAt, maxExpiresAt}` (hai mốc: hết hạn do rảnh và trần tuyệt đối). Nếu đã chạm trần thì trả 401 như phiên hết hạn. Giữ nguyên token (không xoay), vì token đục thu hồi được và xoay vòng làm phức tạp nhiều tab mà ít lợi ích ở mô hình này.
- `Login` và `GET /auth/me` cũng trả `maxExpiresAt` cùng `expiresAt` (hiện chỉ `expiresAt`); `expiresAt` nay nghĩa là mốc hết hạn do rảnh hiện tại.
- Cấu hình mới trong `pkg/auth/config.go`: `sessionIdleTimeout` (mặc định 30m) và `sessionMaxLifetime` (mặc định 24h). `sessionTTL` cũ còn hiểu được: nếu chỉ đặt `sessionTTL`, dùng nó làm `sessionMaxLifetime` và giữ idle mặc định 30m; ghi chú ngừng dùng trong tài liệu. Validate: idle > 0, trần ≥ idle.
- Phiên tạo trước khi nâng cấp (không có `LastActiveAt`) vẫn dùng được đến hết thời hạn Redis hiện có; coi `LastActiveAt = CreatedAt`.
- Giới hạn tần suất: `/auth/refresh` bị giới hạn mềm (mỗi token tối đa vài lần mỗi phút) để tránh dùng nó làm đường dò hay gây tải Redis.
- Thu hồi: đăng xuất, đổi mật khẩu, vô hiệu hóa user giữ nguyên cách hiệu lực hiện nay (kiểm tra trong `Authenticate`).

### 4.2 Giao diện
- `authStore` giữ `expiresAt`, `maxExpiresAt`. Theo dõi **hoạt động thật của người dùng** (chuột, phím, chạm; bỏ qua request nền), giới hạn tần suất ghi nhận.
- Gia hạn ngầm: nếu có hoạt động từ lần gia hạn trước và còn dưới 2/3 thời hạn idle (khoảng 20 phút trong 30), gọi `POST /auth/refresh`. Lỗi mạng tạm thời thì thử lại, không đăng xuất ngay.
- Cảnh báo: còn 2 phút tới hết hạn do rảnh thì hiện hộp thoại có đếm ngược và nút "Tiếp tục đăng nhập" (gọi refresh) và "Đăng xuất". Còn 5 phút tới trần tuyệt đối thì hiện thông báo "phiên kết thúc lúc hh:mm, hãy lưu công việc"; chạm trần thì đăng xuất kèm thông báo rõ ràng, chuyển về trang đăng nhập giữ nguyên đường dẫn để quay lại.
- Nhiều tab: đồng bộ mốc hết hạn và hành động gia hạn/đăng xuất giữa các tab (BroadcastChannel, dự phòng sự kiện `storage`).
- Mọi chuỗi mới có en, de, vi.

## 5. Rủi ro
- **Thay đổi hành vi:** người bỏ tab rảnh hơn 30 phút sẽ phải đăng nhập lại (trước đây là 12 giờ). Đây là mục đích của hết hạn do rảnh, ghi rõ trong CHANGELOG và có thể nới bằng cấu hình.
- Đồng hồ máy khách lệch với máy chủ: UI tính mốc từ `expiresAt` của server trừ đi chênh lệch đo được lúc phản hồi đầu tiên, không tin đồng hồ máy khách.
- Hai tab cùng gia hạn: thao tác gia hạn idempotent nên không lỗi.

## 6. Cách kiểm chứng
1. Backend (đồng hồ giả): `Authenticate` không gia hạn; `refresh` gia hạn đúng `min(idle, trần − đã trôi qua)`; chạm trần trả 401 và không thể vượt; phiên cũ không có `LastActiveAt` vẫn hoạt động; đổi mật khẩu thu hồi phiên sau refresh; cấu hình sai bị từ chối; giới hạn tần suất.
2. UI (đồng hồ giả): hoạt động thật kích hoạt gia hạn, polling nền thì không; cảnh báo 2 phút và 5 phút; hết hạn đăng xuất kèm thông báo; đồng bộ hai tab; lỗi mạng khi refresh thì thử lại.
3. Chạy thật: đăng nhập với `sessionIdleTimeout=2m` và `sessionMaxLifetime=5m` trên stack riêng, xác nhận cảnh báo, gia hạn và đăng xuất đúng thời điểm.
