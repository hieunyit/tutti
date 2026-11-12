# Bản sửa version migration, admission và management API

Các thay đổi giữ nguyên Go, Redis/Asynq, PostgreSQL và S3 SDK hiện tại; không thêm broker hoặc database.

## Version migration

- Work snapshot tách theo replication identity, generation và source object trong namespace `p:repl:objectversion:v2:`.
- Listing và copying dùng cùng object lock ở đích. Listing thay snapshot hoàn chỉnh theo thứ tự oldest → newest; retry không nối trùng hoặc đẩy version mới lên đầu lịch sử.
- Copy giữ lock suốt kiểm tra destination, clear/resume và ghi dữ liệu; chỉ xóa snapshot của chính job sau khi copy hoàn tất. Snapshot của job thất bại còn lại để retry.
- Task cũ chưa có snapshot v2 tự list lại source. Không tự xóa danh sách legacy có thể vẫn thuộc worker cũ.
- Không dọn state bằng prefix chung: namespace v2 nằm dưới prefix version-work cũ. Chỉ dọn legacy bằng công cụ nhận diện schema sau khi đã drain toàn bộ worker cũ.

Kiểm tra: `service/worker/handler/version_work_test.go` và `test/migration/version_history_isolation_test.go`. Integration dùng HTTP S3 nhúng, kiểm đủ bytes/hash của từng version tại hai đích, số lần xuất hiện và latest. Fixture đã sửa HEAD version và thứ tự/pagination version list của GoFakeS3; fixture lưu dữ liệu trong RAM, không phải benchmark backend thật.

## Queue admission

Asynq được giữ ở source snapshot của fork clyso đã ghim, đặt trong `third_party/asynq`; xem `TUTTI-PATCHES.md` tại đó. Patch kiểm tra watermark ngay trong Lua tạo task, thay vì claim/verify ở Go rồi enqueue bằng RPC khác.

- Bao gồm pending, active, scheduled, retry và aggregating theo cùng định nghĩa thống kê queue.
- Các queue không cấu hình `asynq:{queue}:high_watermark` giữ hành vi không giới hạn của upstream.
- TaskID/Unique trùng vẫn idempotent khi queue đầy; từ chối quota không tạo task hoặc unique lock rác.
- QueueService đổi `asynq.ErrQueueFull` thành lỗi backpressure retryable. Không trả nil cho công việc chưa được nhận.
- Redrive archived/completed cũng kiểm tra quota. Batch redrive chỉ chuyển phần còn chỗ, trả số thực sự chuyển; phần dư vẫn archived và vẫn chặn trạng thái DONE.
- Khi hạ watermark dưới backlog đang tồn tại, task mới bị chặn cho tới khi backlog giảm; không tự xóa công việc cũ.

Patch này đảm bảo admission tại commit của task; không tạo transaction phân tán giữa primary S3, version metadata và enqueue.

Chạy test trên Redis thật có sẵn image `redis:7-alpine` (container riêng, 0.25 CPU/96 MiB, tự xóa):

```sh
bash test/scripts/atomic_admission_redis.sh
```

Kiểm tra đang chặn RPC trước Lua commit, nhiều client, TaskID/Unique, các trạng thái queue và redrive archived. Không còn giữ script claim/verify hoặc reflection/unsafe để đọc private field của Asynq. `Inspector.RedisClient()` là accessor chỉ mượn kết nối.

Nâng Asynq sau này phải rebase patch và chạy lại các test trên; không xóa local replacement trước khi có cơ chế admission tương đương.

## Management API và webhook

- Inventory `/tutti/storages` tiếp tục yêu cầu xác thực.
- `/healthz` chỉ báo process còn hoạt động; `/readyz` kiểm tra kết nối Redis meta/config/queue trong timeout. Không trả inventory hoặc lỗi nội bộ. Kubernetes probe dùng hai endpoint này, không dùng API quản trị.
- Webhook disabled trả Unavailable trước khi xử lý event. Enabled/BaseURL được đọc từ effective runtime settings cho các yêu cầu policy mới; startup áp dụng cả giá trị false/rỗng.
- Token và listener/port vẫn là cấu hình startup. Thay BaseURL không tự viết lại subscription đã cấu hình trên backend; phải cập nhật notification target tương ứng.

## Config và triển khai

Interpolation chỉ nhận `${VAR}` trên scalar sau parse YAML. Bcrypt hash và dollar literal được giữ nguyên; quote/backslash/newline trong secret không trở thành cú pháp YAML. Biến được tham chiếu nhưng chưa khai báo làm startup lỗi với tên biến, không in giá trị secret.

Dockerfile copy module metadata của local Asynq trước `go mod download`, rồi copy source ở lớp build như bình thường. Phải rebuild image để chạy bản sửa. Drain/thay toàn bộ bulk worker cũ trước khi nghiệm thu hard watermark; client cũ không chứa Lua admission mới có thể vượt guard trong thời gian chạy lẫn phiên bản. Không cần xóa queue hay dữ liệu để nâng cấp.

Các kiểm tra trên không chứng nhận tải 20 TB/50M object hoặc RPO/RTO trên AWS/Ceph/MinIO thật.
