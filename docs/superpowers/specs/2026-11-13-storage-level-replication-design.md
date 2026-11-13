# Replication cấp storage (không cần user)

Trạng thái: chờ duyệt. Phạm vi: backend (`pkg/api`, `pkg/policy`, `pkg/protectiongroups`, `pkg/objstore`), proto, UI Add replication, `tuttictl`.

## 1. Vì sao làm
Người dùng muốn copy dữ liệu từ storage A sang storage B bằng tài khoản kết nối đã khai báo khi thêm storage. Không cần S3 user trung gian. Protection group đã làm đúng việc này mà không cần user, nhưng màn hình/API "Add replication" vẫn bắt buộc chọn user (`pkg/api/policy_handlers.go:133`, `HasUser` tại `pkg/objstore/credentials.go:415`), nên trên stack chưa có user thì lỗi `404 NotFound` và thông báo chung "An error occurred while processing the list of buckets".

Lời người dùng, nguyên văn ý: replication A sang B không cần user; khi tạo Protection group cũng tạo replication và cũng không cần user.

## 2. Hiện trạng (đã đọc trong code)
- Mọi replication định danh bằng `{user, fromStorage, toStorage[, bucket]}` (`pkg/entity/replication_id.go:34`). `User()` panic nếu rỗng (`:76`).
- Khi sao chép, worker dùng `ResolveS3Credentials(storage, user)`: credential riêng của user nếu có, nếu không dùng tài khoản kết nối của storage (`pkg/objstore/connection.go:112`). User **không phải** nguồn quyền truy cập khi có tài khoản kết nối.
- Protection group dùng danh tính nội bộ `$service` (`pkg/protectiongroups/model.go:21`) khi không có user, và tạo replication bằng `AddUserReplicationPolicy` trực tiếp, bỏ qua `HasUser`/`ValidateReplicationID`.
- Proxy tra policy theo `(user, bucket)` lấy từ access key; nhóm hạ tầng được tra theo storage đích (`LookupStorageGroup`, `pkg/policy/context.go:205-236`) và tự sinh replication từ danh sách thành viên.

## 3. Mục tiêu / không phải mục tiêu / điều tuyệt đối tránh
Mục tiêu:
1. Tạo replication A sang B (toàn bộ bucket hoặc bucket được chọn) **không có trường user** trong UI, API, `tuttictl`.
2. Replication cấp storage và Protection group **dùng chung một luật chống chồng chéo** (mục 6).
3. Lỗi rõ ràng, chỉ đúng đối tượng đang chiếm bucket.

Không làm: đổi mô hình khóa Redis/Postgres, thêm loại policy mới, bỏ replication theo user hiện có (vẫn dùng được cho ai cần lọc theo người dùng).

Điều tuyệt đối tránh: tạo S3 user giả hiện trong danh sách user; làm hỏng replication theo user đang có.

## 4. Các cách đã cân nhắc (người dùng chưa chọn chính thức, mục này là đề xuất)
| Cách | Nội dung | Nhận xét |
|---|---|---|
| **(a) Dùng lại `$service` (đề xuất)** | "Add replication" không user sẽ tạo đúng thứ Protection group đang tạo | Nhỏ nhất, không migration. Không có dữ liệu cần di chuyển nên chi phí của (b)/(c) về dữ liệu bằng 0, nhưng khối lượng sửa code vẫn lớn |
| (b) User tùy chọn, rỗng nghĩa là cấp storage | Đổi định danh, khóa, hàng đợi, khóa chính Postgres | Rất lớn: 44 file UI, ~12 file `tuttictl`, nhiều file Go; `User()` hiện panic khi rỗng |
| (c) Loại policy mới theo cặp storage | Entity, kho, bảng, RPC, vòng đời riêng | Lớn nhất, trùng lặp code pause/resume/status/switch |

## 5. `$service` có khác gì so với không có user (câu trả lời)
Có, **không chỉ là chữ hiện trong Redis/Postgres**. Đã xác minh trong code:

| Chỗ | Ảnh hưởng của `$service` | Cách xử lý đề xuất |
|---|---|---|
| Khóa Redis/Postgres | `p:repl_*:$service…`, `v:$service:…`, cột `user_name='$service'` | Chấp nhận, kín, không ai thấy |
| Tên hàng đợi và ID tác vụ | `AsString()` có tiền tố `$service:` (`pkg/tasks/queue.go:85`), hiện ở công cụ xem hàng đợi | Chấp nhận, ghi trong tài liệu vận hành |
| Nhãn số liệu Prometheus | nhãn `user` mang giá trị `$service` (`pkg/policy/replication_metrics.go`) | Ghi chú trong tài liệu; cardinality không tăng |
| Log, audit log, payload API | trường `user: "$service"` có thể lộ ra | API/UI ánh xạ thành rỗng khi trả về; log giữ nguyên |
| Khóa tuần tự theo user | `AddReplication` lấy `inUserLock(user)` (`policy_handlers.go:148`): mọi thao tác storage-level tuần tự hóa trên một khóa `$service` | Chấp nhận (thao tác tạo ít, không nóng) |
| Luật "replication cả user" chặn "replication theo bucket" | `IsUserReplicationExists(user)` kiểm theo tên user (`replication.go:901`): với một tên `$service` chung, luật này có nguy cơ lan sang **mọi cặp storage** (suy luận, chưa chạy) | Phải kiểm tra bằng test; nếu đúng thì đổi khóa kiểm tra thành `(user, from, to)` riêng cho `$service` |
| Chỗ code xử lý riêng `ServiceUser` | bỏ qua kiểm tra routing (`policy/routing.go:409,431`), proxy gắn replication nhóm (`context.go:232-244`), logic tài khoản dùng chung (`policy/protection_groups.go:27-67`) | Giữ nguyên, kiểm tra không vỡ khi thêm đường tạo mới |
| `HasUser` từ chối `$service` | `ValidateReplicationID` hiện từ chối (`credentials.go:415`) | Cho phép `$service` khi cả hai storage có tài khoản kết nối |
| Trùng tên | một S3 user thật tên `$service` sẽ đụng khóa | Cấm tạo user tên bắt đầu bằng `$` |

Nếu **không có user** hẳn (cách b) thì mọi dòng trong bảng trên biến mất, nhưng đổi lại phải sửa định danh và khóa ở mọi tầng (mục 4).

## 6. Luật chống chồng chéo dùng chung
Luật hiện có (`Group.Overlaps`, `pkg/protectiongroups/model.go:305`): hai nhóm chung ít nhất một storage thành viên bị coi là chồng nếu một bên là toàn storage, hoặc hai bên cùng liệt kê chung ít nhất một bucket. Kiểm tra khi tạo (`store.go:418`) trả `AlreadyExists: scope overlaps group <id>`.

Quy tắc mới:
1. Replication cấp storage được tạo từ "Add replication" được **đăng ký như một nhóm hạ tầng** hoặc phải qua cùng hàm `Overlaps`, để không có hai đường tạo lệch luật.
2. Thông báo lỗi nêu tên đối tượng đang chiếm: "bucket X đã thuộc nhóm/replication Y".
3. Muốn chuyển từ "toàn storage" sang "chọn bucket" phải xóa đối tượng toàn storage trước (không có "loại trừ" trong phạm vi này).

Dữ liệu đã có ở cả A và B khi tạo: luật chồng chéo chỉ xét phạm vi khai báo, không xét dữ liệu. Việc seed ban đầu bỏ qua object giống nhau (`resync.go`). **Chưa kiểm chứng**: object cùng tên khác nội dung (ghi đè hay giữ), object chỉ có ở B (có bị xóa không); sẽ có test chốt hành vi trước khi làm.

## 7. Thay đổi cần làm (cho phần thực thi)
Backend:
- `AddReplication`: `id.user` tùy chọn; rỗng nghĩa là cấp storage, đi qua cùng đường tạo với nhóm hạ tầng, kiểm tra tài khoản kết nối ở cả hai storage, áp luật mục 6. `AvailableBuckets` (`/replication/list-buckets`) chấp nhận không user, liệt kê bucket bằng tài khoản kết nối của storage nguồn.
- Các RPC get/list/pause/resume/delete/switch nhận định danh không user và ánh xạ sang `$service` bên trong.
- Cấm tên user bắt đầu bằng `$` ở nơi tạo S3 user.
- Sửa kiểm tra "user-level vs bucket-level" nếu test chứng minh lan sang nhiều cặp storage.
UI: bỏ ô chọn user và bước dò user khỏi Add replication; thông báo lỗi cụ thể; trang Replication hiện "Storage → Storage" thay cho user.
`tuttictl`: `--user` thành tùy chọn trong các lệnh `repl*`.

## 8. Cách kiểm chứng
1. Test chặn: group chọn bucket khi đã có đối tượng toàn storage trả `AlreadyExists` nêu đúng tên.
2. Test hai cặp storage độc lập (A→B toàn bộ, C→D chọn bucket) không ảnh hưởng nhau.
3. Test seed: bucket đã có ở cả A và B không copy lại object giống nhau; ghi lại hành vi với object khác nội dung hoặc chỉ có ở B.
4. Chạy lại trên stack thật có hai storage FPT và tài khoản kết nối, không có S3 user nào.

## 9. Việc để người thực thi quyết
Tên hiển thị của replication cấp storage trong UI; cách ánh xạ trường `user` trong phản hồi API; có đưa vào thư mục `docs/tutti` mục vận hành về nhãn `$service` hay không.

## 10. Rủi ro còn mở
- Nghi vấn lỗi seed ra 0 bucket với `$service` ở replication cấp user (`scope.Filter` trả rỗng khi storage dùng tài khoản dùng chung). Phải xác minh bằng test **trước** khi làm phần còn lại.
- Luật "replication toàn user chặn theo bucket" có thể lan sang mọi cặp storage dưới một tên `$service` chung.
