# Kiểm chứng review bằng test — 2026-11-09

> Đây là kết quả trước khi sửa. Trạng thái sau sửa và lệnh regression test hiện tại nằm trong [review-fixes-2026-11-09.md](./review-fixes-2026-11-09.md). Các test Go đã bỏ build tag, test UI đã chuyển vào bộ test thường.

Code kiểm tra: HEAD `bf9cc7659805ca9bb1f757b4362d5b809c8c0ba5` và toàn bộ working tree hiện tại, gồm thay đổi UI chưa commit. Không fetch remote, không sửa production code. Ba agent kiểm tra bảo mật, replication/failover và UI; agent chính kiểm tra context, retry budget, probe và chạy lại bộ test tổng hợp.

## Bộ kiểm tra có sẵn

| Kiểm tra | Kết quả |
|---|---|
| `go test -count=1 ./pkg/... ./service/...` | Đạt |
| `go vet ./...` | Đạt |
| Race detector các package quan trọng (lệnh bên dưới) | Đạt |
| UI unit test | 104/104 đạt |
| UI type-check | Đạt |
| UI ESLint | 7 lỗi, giống review trước |
| UI build | Đạt; JS chính 1.653,19 kB, gzip 473,88 kB |

## Test tái hiện và kết luận

Các test mới yêu cầu hành vi đúng. Chúng đang thất bại vì code chưa đáp ứng; không phải lỗi cài đặt hay fixture không chạy được. Go test được đặt sau build tag `review_verify`; UI test nằm ngoài mẫu mặc định. Vì vậy bộ test thông thường không bị các test đang đỏ này ảnh hưởng.

| Mục review | Kết quả thực nghiệm | Test |
|---|---|---|
| 1. Policy cache fail-open | Cold load đang chờ hoặc Redis lỗi lần đầu đều trả `found=false`, là nhánh middleware cho toàn quyền | `pkg/accesspolicy/security_review_test.go` |
| 2. Xóa replica khi nguồn lỗi | Endpoint nguồn trả 403 AccessDenied; Sync trả nil và backend nhận đúng 1 DELETE | `service/worker/syncer/delete_review_test.go` |
| 3. Không kiểm tra freshness chữ ký | SigV4 và SigV2 với thời gian hiện tại đạt control; cũ 48 giờ và tương lai 48 giờ đều được authenticate chấp nhận | `service/proxy/auth/security_review_test.go` |
| 4. Failover kẹt sau pause | Chèn lỗi transaction routing sau pause bằng Redis hook; hạ tầng phục hồi rồi RunOnce lại: routing vẫn main, replication paused=true | `service/proxy/failover/partial_review_test.go` |
| 5. Mất caller context | Caller context đã canceled nhưng request tới RoundTripper không canceled | `pkg/s3client/context_review_test.go` |
| 6. Webhook ACK lỗi queue | S3 và Swift đều gọi Replicate 1 lần nhận lỗi queue nhưng handler trả nil | `pkg/api/webhook_review_test.go` |
| 7. Retry budget không chặn dependency | Replicate vẫn được gọi 1 lần sau khi budget hết; khi budget còn thì context truyền xuống không có deadline đó | `service/proxy/router/budget_review_test.go` |
| 8. Session giữ quyền cũ | Login admin rồi tạo service mới dùng cùng Redis: user đã xóa vẫn authenticate; hạ xuống viewer vẫn trả admin | `pkg/auth/security_review_test.go` |
| 9. Probe chạy chồng | Giữ probe đầu không tuân thủ context; sau timeout probe thứ hai bắt đầu khi probe đầu chưa kết thúc | `pkg/health/checker_review_test.go` |
| 10. Monitoring response cũ | Response poll trước ghi đè proxy mới; response đang chờ điền lại state sau reset | `ui/review-tests/monitoring.review.ts` |
| 11. Bundle lớn | Xác nhận bằng production build; đây là số đo kích thước, chưa phải benchmark trải nghiệm | Build output |
| 12. ESLint | Xác nhận 7 lỗi hiện tại | ESLint output |

Không có phát hiện nào trong 12 mục bị bác bỏ trong lần kiểm tra này. Cần giữ giới hạn của bằng chứng: các test kiểm tra chức năng thật với dependency được kiểm soát, không phải E2E storage production. Mục 9 chỉ xảy ra khi probe không tuân thủ cancellation; chưa chứng minh các probe hiện dùng trong production bị treo. Mục 11 là cơ hội tối ưu, không tự nó chứng minh ứng dụng chậm. Mục 12 là lỗi kiểm tra CI, không phải lỗi runtime. Test race đạt không bác bỏ các race logic tại cold cache hoặc response bất đồng bộ.

## Lệnh tái hiện

Từ thư mục gốc (lệnh Go tái hiện dự kiến exit 1 khi các lỗi còn tồn tại):

```bash
go test -tags review_verify -run TestReview -count=1 -v \
  ./pkg/accesspolicy ./pkg/auth ./pkg/api ./pkg/health ./pkg/s3client \
  ./service/proxy/auth ./service/proxy/failover ./service/proxy/router \
  ./service/worker/syncer
```

UI tái hiện dự kiến 2 test fail:

```bash
cd ui
npm exec -- vitest --run --config review-tests/vitest.config.ts
```

Bộ test Go có sẵn:

```bash
go test -count=1 ./pkg/... ./service/...
go vet ./...
go test -race -count=1 ./pkg/auth ./pkg/accesspolicy ./pkg/proxyaccount \
  ./pkg/health ./pkg/policy ./service/proxy/... ./service/worker/syncer
```

UI có sẵn (từ `ui/`):

```bash
npm run test:unit -- --run
npm run lint:types
npm run lint:eslint
npm run build
```

## ESLint hiện tại

- `ui/src/components/tutti/replications/ReplicationsList/ReplicationsList.vue:28`: import order.
- `ui/src/components/tutti/set-credential/CredentialFormTile/CredentialFormTile.vue:31` và `:32`: hai biến storage/user không dùng.
- `ui/src/components/tutti/accounts/ProxyAccountKeysTable.vue:92`: thiếu dòng trống.
- `ui/src/stores/tuttiAddDiffReportStore.ts:20`: import order.
- `ui/src/stores/tuttiAddReplicationStore.ts:19`: import order.
- `ui/src/stores/tuttiSetCredentialStore.ts:22`: import order.

## Giới hạn và ưu tiên

Chưa chạy E2E Docker/MinIO, load test, trình duyệt thật hoặc quét CVE dependency. Chưa sửa production code. Các lỗi mở localhost/cache do sandbox đã được chạy lại với quyền phù hợp; kết luận trên dựa trên lần chạy hoàn tất.

Ưu tiên sửa: xóa replica khi nguồn lỗi; policy cache fail-open; chữ ký không kiểm tra thời gian; failover dở dang; caller context. Dùng các test đang đỏ làm regression test trong quá trình sửa.
