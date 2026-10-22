# Studio Album — Đặc tả v1 (đã chốt)

## 1. Nền tảng
- Synology DSM 7.x, CPU x86_64. Đóng gói `.spk` cài qua Package Center (Manual Install), **không Docker**.
- Backend: **Go**, một file thực thi duy nhất, không phụ thuộc runtime trên NAS. CSDL **SQLite**. Frontend **Vue 3** build tĩnh, nhúng vào binary.
- Realtime (chat, bình luận, cập nhật chọn ảnh): WebSocket.
- Truy cập từ Internet: domain riêng, dùng DSM Reverse Proxy + SSL Let's Encrypt. App chạy ở cổng 8686.
- Ảnh xem trước: app tự tạo thumbnail (~400px) và bản preview (~2048px) vào cache riêng, **không động vào file gốc**.

## 2. Phần 1 — Admin Console (giống Synology Drive Admin Console)
- Hiện trong menu chính DSM, chỉ admin DSM mở được (dùng phiên đăng nhập DSM). Dự phòng: `http://NAS:8686/console/` trong LAN, đăng nhập bằng tài khoản admin DSM.
- **Share folder**: liệt kê các share folder của NAS, báo app có quyền đọc hay chưa; admin bật/tắt share nào được dùng cho album. Cấp quyền bằng Control Panel (user hệ thống `StudioAlbum`, Read only là đủ) — Console hướng dẫn từng bước.
- Trạng thái dịch vụ: cổng, URL public, dung lượng cache thumbnail, nút dọn cache / quét lại thư mục.
- Tạo/đặt lại tài khoản **Admin gốc** của ứng dụng chính.
- Tải bản sao lưu CSDL, dọn cache, giới hạn dung lượng preview, xem log, hướng dẫn cấu hình tên miền.

## 3. Phần 2 — Ứng dụng chính

### 3.1 Phía studio (tài khoản riêng của app)
- **Phân quyền tuỳ biến**: Admin tạo nhóm quyền và tick từng quyền, ví dụ: quản lý khách, tạo/sửa album, xem mọi album hay chỉ album được gán, chat, xem ảnh đã chọn, đánh dấu đã edit xong, mở khóa lựa chọn, cài đặt hệ thống, quản lý nhân viên.
- **Khách hàng**: SĐT (định danh đăng nhập), tên, mật khẩu do studio cấp (tạo ngẫu nhiên hoặc tự đặt, có nút đặt lại), ghi chú nội bộ.
- **Album** = 1 thư mục trong share folder đã được bật ở Console (chọn bằng trình duyệt thư mục).
  - Gán một hoặc nhiều khách (cô dâu + chú rể cùng vào một album).
  - Link chỉ xem (không cần đăng nhập, bật/tắt, có hạn).
  - **Cài đặt album**: số ảnh edit tối đa (0 = không giới hạn); cho phép tải (gốc/tất cả/đã chọn); ngày hết hạn (hết hạn thì khóa truy cập); nhân viên edit được gán; hạn trả ảnh; ghi chú nội bộ; ghi chú hiển thị cho khách.
  - Thư mục con **`Final`** (đổi được trong Cài đặt) → khách thấy tab "Ảnh hoàn thiện".
  - Trạng thái: Đang chọn → Đã gửi (khóa) → Đang edit → Đã trả ảnh. Studio mở khóa được.
- **Màn hình làm việc của editor**: danh sách ảnh khách chọn edit kèm bình luận từng ảnh; xuất danh sách tên file (copy/CSV) để lọc trong Lightroom; đánh dấu từng ảnh đã xong.
- Bảng tổng quan: album sắp hết hạn trả, album khách vừa gửi, tin nhắn chưa đọc.
- Cài đặt thương hiệu: logo, tên studio, màu chủ đạo.

### 3.2 Phía khách hàng
- Đăng nhập bằng SĐT + mật khẩu. Thấy **tất cả album** của mình.
- Lưới ảnh tối ưu cho điện thoại, xem toàn màn hình, vuốt qua lại.
- **Tầng 1 — Yêu thích (tim)**: không giới hạn. Có bộ lọc Tất cả / Yêu thích / Chưa xem.
- **Tầng 2 — Chọn edit**: chọn từ danh sách yêu thích (nút ✎ trên ảnh), bộ đếm `x / N`, không vượt quá N. Nút **Gửi studio** → khóa lựa chọn.
- Ghi chú chung cho cả album (tông màu, phong cách…).
- **Bình luận trên từng ảnh** (2 chiều với studio).
- **Chat realtime theo album**.
- **Tải xuống**: một ảnh, ảnh yêu thích, ảnh chọn edit, tất cả (ZIP stream, không tạo file tạm). Tải file JPG gốc.
- Song ngữ Việt/Anh, PWA (Add to Home screen).

## 4. Quy mô thiết kế
- 500–2000 ảnh/album, 50–200 album/tháng → thumbnail tạo nền theo hàng đợi, phân trang/lazy-load lưới ảnh.

## 5. Bảo mật
- Mật khẩu hash bằng bcrypt/argon2; giới hạn số lần đăng nhập sai theo SĐT/IP.
- Kiểm tra quyền và đường dẫn ở mọi API file (chống `../`), chỉ phục vụ file nằm trong share folder đã bật.
- Cookie phiên HttpOnly + Secure qua HTTPS.

## 6. Chưa làm trong v1 (đã thống nhất bỏ)
- Thông báo ra ngoài (Telegram/Zalo/email) — v1 thông báo trong app (chuông, âm báo, notification trình duyệt).
- Gửi file trong chat, watermark, ảnh RAW/HEIC, phụ phí ảnh vượt. Quyền tải ảnh bật/tắt theo từng album.
