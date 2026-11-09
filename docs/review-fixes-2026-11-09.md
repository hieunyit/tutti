# Bản sửa sau kiểm chứng review — 2026-11-09

Các thay đổi được thực hiện trên HEAD `bf9cc76` cùng working tree đang có. Phần UI (mục 10–12) đã nằm trong commit J2 `7c5b36c`/J3 `94880a8`; phần backend được commit riêng sau J3. Chưa deploy.

## Thay đổi

1. Policy cache chưa tải thành công trả policy từ chối truy cập; chỉ cấp quyền mặc định khi đã biết chắc user không có policy. Cache phục hồi sau khi Redis hoạt động lại.
2. Syncer chỉ xóa replica khi nguồn xác nhận 404 NoSuchKey/NoSuchVersion. Lỗi 403/503 hoặc lỗi mạng được trả về để retry.
3. SigV4 và SigV2 dạng header giới hạn lệch thời gian ±15 phút; SigV4 còn đối chiếu ngày credential scope. Presigned giữ kiểm tra thời hạn riêng.
4. Failover lưu `failover_pending` trước khi pause; đổi routing và xóa marker trong cùng transaction. Sau lỗi/restart, controller tiếp tục được. Replication operator đã pause vẫn không được tự động promote; Pause/Resume chủ động xóa marker.
5. Request gửi storage giữ context của caller để cancellation/deadline có hiệu lực.
6. Webhook S3/Swift trả lỗi khi queue hoặc context-building lỗi, cho phép sender retry. Record đầu vào không hợp lệ vẫn được bỏ qua.
7. Replication retry truyền deadline xuống dependency, không gọi khi budget đã hết và dừng xử lý task tiếp theo khi hết budget.
8. Session từ chối user đã bị xóa; role luôn lấy từ cấu hình user hiện tại sau restart.
9. Health checker giữ tối đa một probe đang chạy cho mỗi target. Probe treo tiếp tục được ghi nhận timeout để mở breaker; khi probe thực sự kết thúc, các tick sau có thể probe lại. Kết quả thành công trả về muộn sau timeout không đóng breaker.
10. Monitoring dùng AbortController và bảo vệ request đang chạy; response/lỗi thuộc phiên cũ không thể cập nhật state sau pause/stop/reset/restart.
11. Page/layout được tải theo route. Bundle JS chính giảm 1.653,19 xuống 1.043,91 kB; gzip giảm 473,88 xuống 298,89 kB, khoảng 37%. Shared entry vẫn trên 500 kB; chưa đo trải nghiệm trình duyệt.
12. Sửa toàn bộ 7 lỗi ESLint đã ghi nhận.

## Regression test

Các test Go `*_review_test.go` đã bỏ build tag và chạy cùng bộ test mặc định. Test UI tạm đã chuyển vào `ui/src/stores/__tests__/tuttiMonitoring.spec.ts`; bổ sung test truyền AbortSignal tại service.

Test kiểm tra cả hành vi an toàn lẫn phục hồi: cache Redis lỗi rồi phục hồi, chữ ký hiện tại/±10 phút hợp lệ và ±48 giờ bị từ chối, 403/503 không xóa nhưng 404 vẫn xóa, failover qua controller mới và operator pause, probe treo không overlap nhưng vẫn mở breaker và phục hồi.

## Kiểm tra

- `go test -count=1 ./pkg/... ./service/...`: đạt.
- `go vet ./...`: đạt.
- Race detector trên các package auth, policy, API, store, health, S3 client, proxy và syncer: đạt.
- UI unit tests: 110/110 đạt.
- UI type-check, ESLint và production build: đạt.
- `git diff --check`: sạch.

Kiểm tra lại trên `94880a8` (sau J3) trước khi commit backend: Go build/vet/test và race detector đạt; UI `npm run lint` đầy đủ (type-check, ESLint, Oxlint, Stylelint, Prettier) đạt; UI unit tests 120/120 đạt.

Lệnh regression trọng tâm:

```bash
go test -run TestReview -count=1 ./pkg/accesspolicy ./pkg/auth ./pkg/api \
  ./pkg/health ./pkg/s3client ./service/proxy/auth ./service/proxy/failover \
  ./service/proxy/router ./service/worker/syncer
```

Lệnh kiểm tra toàn bộ:

```bash
go test -count=1 ./pkg/... ./service/...
go vet ./...
go test -race -count=1 ./pkg/auth ./pkg/accesspolicy ./pkg/api ./pkg/store \
  ./pkg/proxyaccount ./pkg/health ./pkg/policy ./pkg/s3client \
  ./service/proxy/... ./service/worker/syncer
cd ui
npm run test:unit -- --run
npm run lint:types
npm run lint:eslint
npm run build
```

## Lưu ý vận hành

- Đồng hồ client S3 và proxy cần đồng bộ trong cửa sổ 15 phút; request quá cũ sẽ trả RequestTimeTooSkewed.
- Khi policy cache chưa được khởi tạo và Redis lỗi, request sẽ bị từ chối thay vì được toàn quyền.
- Webhook retry cả batch có thể enqueue lại record đã xử lý trước lỗi; xử lý theo semantics ít nhất một lần qua object reconciliation hiện có.
- Field Redis `failover_pending` mới không cần migration: thiếu field tương đương false.
- Chưa chạy E2E Docker/MinIO, benchmark tải, trình duyệt thật hoặc quét CVE dependency.
