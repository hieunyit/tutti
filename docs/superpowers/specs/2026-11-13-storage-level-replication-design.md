# Replication cấp storage, không có user (thiết kế đã duyệt hướng)

Trạng thái: hướng đã được người dùng chọn (cách b + luật chống chồng chéo chung). Phạm vi: backend (`pkg/entity`, `pkg/store`, `pkg/policy`, `pkg/protectiongroups`, `pkg/api`, `pkg/objstore`, `service/worker`, `service/proxy`), proto, UI, `tuttictl`, tài liệu.

## 1. Vì sao
Replication từ storage A sang B dùng tài khoản kết nối của storage, không cần S3 user trung gian. Hiện "Add replication" bắt buộc `user` (`pkg/api/policy_handlers.go:133`, `HasUser` ở `pkg/objstore/credentials.go:415`), nên khi chưa có user thì lỗi `404` và thông báo chung khó hiểu. Protection group đã copy A sang B không cần user, nhưng phải dùng danh tính giả `$service` (`pkg/protectiongroups/model.go:21`). Người dùng yêu cầu bỏ user (và bỏ cả `$service`) khỏi mô hình replication cấp storage.

## 2. Quyết định của người dùng
1. **Cách (b):** user trở thành tùy chọn; **rỗng nghĩa là replication cấp storage**. Bỏ danh tính `$service` khỏi toàn bộ code.
2. **Luật chống chồng chéo chung:** khi tạo replication hoặc group (chọn bucket hoặc chọn tất cả), nếu đã có replication nào phủ cùng bucket thì phải **dừng lại và báo lỗi**, không tạo gì cả.
3. Hiện không có dữ liệu hay tác vụ đang chạy, nên không cần migration dữ liệu.

## 3. Mục tiêu / không làm / tránh tuyệt đối
Mục tiêu: tạo replication A→B (toàn bộ bucket hoặc bucket chọn) không có trường user ở UI, API, `tuttictl`; Protection group và Add replication dùng cùng một luật chống chồng chéo; lỗi chỉ đúng đối tượng đang chiếm bucket.
Không làm: bỏ replication theo user (vẫn dùng được để lọc theo người dùng); loại trừ bucket ("toàn storage trừ X"); đổi mô hình failover.
Tuyệt đối tránh: tạo dở dang (một phần replication) khi phát hiện chồng; hai đường tạo lệch luật; làm hỏng replication theo user đang có.

## 4. Thiết kế

### 4.1 Định danh và khóa
- `UniversalReplicationID` (`pkg/entity/replication_id.go`): `user` được phép rỗng. `User()` không panic; `IsEmpty()` không coi user rỗng là rỗng. `AsString()` và `UniversalIDFromString` phải mã hóa/giải mã được user rỗng (token đầu rỗng) mà không nhập nhằng với dạng bucket; người thực thi chọn mã hóa, miễn có test round-trip cho 4 dạng (user/cấp storage × cấp user/cấp bucket). Tên hàng đợi (`pkg/tasks/queue.go:85`) dùng cùng mã hóa.
- Redis: các hàm dựng khóa (`pkg/store/replication_stores.go`, `replication_generation_store.go`, `routing_stores.go`, `pkg/meta/version_keys.go`) chấp nhận user rỗng. Postgres: cột `user_name` đã `NOT NULL` và chuỗi rỗng hợp lệ, **không đổi schema**; kiểm tra khóa chính không đụng với dòng bucket (`'' ` dùng cho bucket ở `routes`/`route_blocks`).
- Xóa hằng `protectiongroups.ServiceUser` và mọi chỗ xử lý riêng nó: `pkg/policy/routing.go:409,431`, `pkg/policy/context.go:232-244,513,521`, `pkg/policy/protection_groups.go:27-67`, `pkg/protectiongroups/credentials.go:19`. Thay bằng điều kiện `user == ""`.

### 4.2 Xác thực quyền truy cập
`ResolveS3Credentials(storage, "")` rơi về tài khoản kết nối (`pkg/objstore/connection.go:112`). `HasUser`/`ValidateReplicationID` với user rỗng: hợp lệ khi **cả hai storage** có tài khoản kết nối; ngược lại trả lỗi nêu rõ storage nào thiếu tài khoản kết nối. Không tạo "danh tính" nào trong `s3_identities`.

### 4.3 API
- `AddReplication`, `GetReplication`, `ListReplications`, `Pause/Resume/DeleteReplication`, `Switch*`, `AvailableBuckets` (`/replication/list-buckets`): `user` tùy chọn. Rỗng = cấp storage, bucket được liệt kê bằng tài khoản kết nối của storage nguồn.
- Khóa tuần tự theo user (`inUserLock`) bỏ qua khi user rỗng; thay bằng khóa theo cặp storage đã có (`StorageLocker`) cộng khóa phạm vi ở 4.4.
- Phản hồi không bao giờ trả `$service`; user rỗng trả là chuỗi rỗng.

### 4.4 Luật chống chồng chéo chung (cốt lõi)
Định nghĩa **phạm vi (scope)** của một đối tượng: một tập **cạnh có hướng** `from→to` giữa các storage và một tập bucket (liệt kê cụ thể hoặc "tất cả"). Mọi đối tượng sinh replication đều có scope: Protection group, replication cấp storage, replication theo user, replication theo bucket. Một group với các thành viên M đóng góp **mọi cạnh có hướng giữa các thành viên** (vì switchover có thể đảo primary), không chỉ các cạnh của primary hiện tại.

Hai scope **chồng** khi tồn tại một cạnh `e1` của bên này và một cạnh `e2` của bên kia thỏa **một** trong hai:
1. **Cùng đích:** `e1.to == e2.to` (gồm cả trùng hẳn `A→B` với `A→B`; hai nguồn cùng ghi vào một đích sẽ ghi đè lẫn nhau).
2. **Ngược chiều:** `e1.from == e2.to` và `e1.to == e2.from` (`A→B` với `B→A` sẽ copy qua lại mãi).

**và** tập bucket giao nhau (hoặc một bên là "tất cả"). Danh tính user không làm hai scope tách rời, vì tên bucket S3 định danh dữ liệu vật lý.

**Làm rõ theo user (bổ sung sau khi triển khai):** "tất cả bucket" của một user thật chỉ có nghĩa là mọi bucket **của user đó**, không phải mọi bucket của storage. Vì vậy hai đối tượng thuộc **hai user thật khác nhau** chỉ coi là chồng khi danh sách bucket **liệt kê cụ thể** của chúng giao nhau; "tất cả" của user này không chồng với "tất cả" hay bucket của user kia. Đối tượng **cấp storage** (user rỗng) thật sự phủ mọi bucket của storage, nên vẫn chồng với mọi đối tượng cùng cạnh theo luật ở trên. Điều này giữ nguyên hành vi hiện có của replication theo từng user (nhiều user cùng nhân bản sang một storage đích vẫn được).

Được phép (không coi là chồng): cùng nguồn khác đích (`A→B` và `A→C` cho cùng bucket, tức fan-out bằng replication rời), và chuỗi nối tiếp (`A→B` rồi `B→C`). Hai replication khác đích và không ngược chiều (ví dụ `A→B` và `C→D`) không ảnh hưởng nhau.

**Giới hạn của kho lưu trữ:** kho policy chỉ giữ **một storage nguồn** cho mỗi (user, bucket) (`p:repl_bucket_from:<user>`) và mỗi user (`p:repl_user_from`); mọi replication cấp storage dùng chung user rỗng. Vì vậy hai đối tượng **cùng user** (cùng rỗng, hoặc cùng một user thật, ví dụ alice `A→B` và alice `C→D`) có **nguồn khác nhau** (`e1.from != e2.from`) mà cùng bucket (danh sách liệt kê giao nhau, hoặc cả hai đều "tất cả") sẽ ghi đè nguồn của nhau và để lại trạng thái hỏng; chúng bị coi là chồng (lý do `source_storage`, thông báo "storage-level replications must share one source storage per bucket", hoặc "replications of user \"alice\" must share ..." khi có user; hai user thật khác nhau vẫn độc lập), dù đích khác nhau và không ngược chiều. Cùng nguồn khác đích vẫn được phép; "tất cả" với danh sách bucket cụ thể vẫn được phép vì proxy tra policy bucket trước policy toàn bucket. Hệ quả: chuỗi `A→B`, `B→C` chỉ được phép khi bucket rời nhau, và group (mọi cặp có hướng) áp dụng cùng luật khi không có user. Giải pháp dài hạn là đổi khóa kho theo storage nguồn (re-keying); chưa làm.

Hành vi:
1. Kiểm tra chồng chạy **trước khi tạo bất cứ thứ gì**, và nằm trong cùng khóa (một khóa phạm vi toàn cục cho thao tác tạo) với việc ghi để hai lần tạo đồng thời không cùng lọt qua. Phát hiện chồng thì dừng hẳn, không để lại trạng thái dở dang, trả `AlreadyExists` kèm tên đối tượng và bucket cụ thể đang bị chiếm.
2. Áp dụng cho cả: tạo group (chọn bucket hoặc tất cả), tạo replication (chọn bucket hoặc tất cả), và cập nhật group.
3. `AvailableBuckets` đánh dấu bucket đã được phủ bởi bất kỳ replication hay group nào (không chỉ replication cùng user), để UI không cho chọn.
4. Một hàm duy nhất (ví dụ `scope.Conflicts(existing, proposed)`) dùng cho mọi đường; không để `Group.Overlaps` và kiểm tra của replication tách riêng.
5. Muốn đổi từ "tất cả" sang "chọn bucket" phải xóa đối tượng toàn storage trước.

Dữ liệu đã có ở cả A và B khi tạo: luật chỉ xét phạm vi khai báo. Seed ban đầu bỏ qua object giống nhau. **Chưa kiểm chứng:** object cùng tên khác nội dung (ghi đè hay giữ) và object chỉ có ở B; phải có test chốt hành vi.

### 4.5 Proxy, worker, webhook
- Proxy: `BuildProxyContext(user, bucket)` (`pkg/policy/context.go:120`) vẫn tra theo user thật (nếu client có) và **thêm** tra replication cấp storage theo storage đích + bucket, thay cho đường tổng hợp `$service`; kết quả quyết định có kích hoạt replication tức thời cho ghi qua proxy.
- Worker: các handler dùng `ID.User()` (`service/worker/handler/*`, `copy/copy.go`, `syncer/syncer.go`) phải chạy được với user rỗng bằng `AsCommon(storage, "")` rơi về tài khoản kết nối.
- Webhook: `ConfigurationId` của thông báo S3 mã hóa user (`pkg/api/webhook_handlers.go:204-225`); cần cách mã hóa cho user rỗng.
- Tác vụ có trường `User` riêng (diff, `pkg/tasks/tasks.go:249-295`): hỗ trợ user rỗng cho cặp storage.

### 4.6 UI và tuttictl
- UI: Add replication **không có ô chọn user** và không dò user. Bước chọn bucket liệt kê bằng tài khoản kết nối, đánh dấu và vô hiệu hóa bucket bị chiếm kèm lý do. Danh sách Replication hiển thị "A → B" cho dòng cấp storage. Khi có replication theo user thật vẫn hiện user.
- Gỡ phần "chọn tenant user" (`tuttiAddReplicationStore`: `users`, `eligibleUsers`, `selectedUser`) và các nhãn liên quan.
- `tuttictl`: `--user` tùy chọn ở các lệnh `repl*`.

## 5. Các cách đã cân nhắc
(a) dùng lại `$service`: nhỏ nhất, nhưng để lại danh tính giả lộ ra ở hàng đợi, nhãn số liệu, log, khóa tuần tự dùng chung và nguy cơ luật user-level lan sang mọi cặp storage — người dùng bác. (c) loại policy mới theo cặp storage: sạch nhưng trùng lặp toàn bộ vòng đời. **Đã chọn (b)** vì mô hình đồng nhất; chi phí migration bằng 0 do chưa có dữ liệu.

## 6. Cách kiểm chứng
1. Test round-trip `AsString`/`UniversalIDFromString` cho user rỗng ở mọi dạng.
2. Test chồng chéo (bảng chân trị): tất cả×tất cả, tất cả×bucket, bucket giao nhau, bucket rời; **cùng đích** (`A→B`/`A→B`, `A→B`/`C→B`) bị chặn; **ngược chiều** (`A→B`/`B→A`) bị chặn; **cùng nguồn khác đích** (`A→B`/`A→C`) và **chuỗi** (`A→B`/`B→C`) được phép; group×group, group×replication (cạnh của group là mọi cặp có hướng giữa thành viên), replication×replication, cấp storage×theo user; thông báo nêu đúng đối tượng và bucket.
3. Test không dở dang: lỗi chồng không để lại khóa/policy nào; hai lần tạo đồng thời cùng bucket chỉ một lần thành công.
4. Test worker copy và seed với user rỗng bằng tài khoản kết nối; test object cùng tên khác nội dung, object chỉ có ở B.
5. Test proxy: ghi vào bucket thuộc replication cấp storage kích hoạt đúng một replication.
6. Chạy lại trên stack thật: hai storage FPT, không có S3 user nào, Add replication thành công và bị chặn đúng khi chọn bucket trùng.

## 7. Rủi ro
- Chạm gần như mọi tầng (khoảng 44 file UI không test, ~12 file `tuttictl`, nhiều file Go): chia theo lớp, mỗi lớp có test trước.
- Mã hóa chuỗi ID/hàng đợi với user rỗng dễ nhập nhằng; test round-trip bắt buộc.
- `IsUserReplicationExists` và mọi kiểm tra "theo tên user" phải được duyệt lại để không coi user rỗng là một user duy nhất chung cho mọi cặp storage.

## 8. Để người thực thi quyết
Mã hóa cụ thể của ID rỗng; tên hàm chung `Conflicts`; cách hiển thị dòng cấp storage trong danh sách.
