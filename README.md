# Studio Album

Ứng dụng cho studio ảnh chạy trên Synology DSM 7 (x86_64), cài trực tiếp bằng file `.spk`, không cần Docker. Khách đăng nhập bằng số điện thoại để xem album, thả tim, chọn ảnh edit, ghi chú, chat với studio và tải ảnh.

Gồm 2 phần:

| Phần | Ai dùng | Mở ở đâu |
|---|---|---|
| **Studio Album Console** | Quản trị viên NAS | Menu chính DSM, giống Synology Drive Admin Console. Dự phòng: `http://IP-NAS:8686/console/` (chỉ LAN) |
| **Studio Album** (ứng dụng chính) | Nhân viên studio: `/studio/`. Khách: `/` | `http://IP-NAS:8686`, hoặc tên miền qua Reverse Proxy |

## 1. Cài đặt lên NAS

1. Mở **Package Center → Manual Install**, chọn `dist/StudioAlbum-x86_64-1.0.0.spk`, đồng ý cài gói bên thứ ba.
2. Mở **Studio Album Console** từ menu chính DSM:
   - Tab **Tài khoản quản trị**: tạo tài khoản chủ (toàn quyền).
   - Tab **Share folder**: với share chứa ảnh (vd `photo`), cấp quyền đọc cho user hệ thống của app theo hướng dẫn ngay trên trang (Control Panel → Shared Folder → Edit → Permissions → *System internal user* → `StudioAlbum` → Read only). Sau đó bật **Dùng cho album**.
3. Mở `http://IP-NAS:8686/studio/`, đăng nhập bằng tài khoản chủ.
4. Tên miền: làm theo tab **Cài đặt & tên miền** trong Console (Reverse Proxy HTTPS → `localhost:8686`, thêm Custom Header **WebSocket**, chứng chỉ Let's Encrypt). Điền địa chỉ public vào Console hoặc vào Cài đặt của app.

## 2. Cập nhật tự động qua Package Center

Cài lần đầu bằng Manual Install (tải file `.spk` ở trang **Releases** của repo GitHub). Sau đó làm **một lần**:

**Package Center → Settings → Package Sources → Add** — Name: `Studio Album`, Location: `http://127.0.0.1:8686/pkg-source`

Từ đó, mỗi khi có Release mới trên GitHub, Package Center hiện nút **Update** cho Studio Album. Dữ liệu (album, khách, tin nhắn) được giữ nguyên. Tab **Cập nhật** trong Console cho biết phiên bản đang chạy và bản mới nhất.

Phát hành bản mới (người phát triển):

```bash
git tag v1.0.1
git push origin v1.0.1
```

GitHub Actions tự chạy test, build `.spk` và tạo Release, NAS thấy bản mới trong vòng ~10 phút. Nếu bản mới lỗi khiến app không chạy: tải `.spk` bản trước trong Releases và Manual Install.

## 3. Quy trình làm việc

1. Studio chép ảnh vào một thư mục, vd `photo/2026/10_CuoiHoi_Lan_Minh`.
2. **Album → Tạo album**: chọn thư mục, gán khách (tạo khách mới ngay trong form; app sinh mật khẩu và tin nhắn mẫu để gửi qua Zalo/SMS), đặt số ảnh edit, hạn trả ảnh, hạn truy cập, quyền tải, nhân viên edit.
3. Khách chọn ảnh:
   - Thả tim không giới hạn để lọc dần.
   - Trong tab *Yêu thích*, bấm ✎ để chọn ảnh edit, tối đa N ảnh.
   - Ghi chú trên từng ảnh, chat, rồi bấm **Gửi studio** để khóa lựa chọn.
4. Studio nhận thông báo realtime. Tab **Danh sách edit** có các nút:
   - **Chép tên file**: dán vào Library Filter của Lightroom.
   - **Xuất CSV**: kèm ghi chú của khách.
   - **Tải ZIP** và **đánh dấu đã edit**: từng ảnh.
5. Ảnh đã chỉnh sửa chép vào thư mục con **`Final`** của album. Khách thấy tab *Ảnh hoàn thiện* để xem và tải. Chuyển trạng thái album sang **Đã trả ảnh**.

Trạng thái album: *Khách đang chọn → Khách đã gửi (khóa) → Đang chỉnh sửa → Đã trả ảnh*. Người có quyền "Đổi trạng thái" có thể mở khóa cho khách chọn lại.

## 4. Cấu trúc mã nguồn

```
backend/            Go, một binary duy nhất (SQLite thuần Go, WebSocket, tạo thumbnail)
  web/              Frontend đã build, nhúng vào binary
  tools/            mkicons (icon), mkspk (đóng gói .spk), gensample (ảnh mẫu để test)
frontend/           Vue 3 + Vite: index.html (khách), studio.html (studio), console.html
package/            Metadata gói DSM: INFO, scripts, conf, ui/ (menu DSM + console + api.cgi)
build.ps1 / build.sh  Build toàn bộ → dist/StudioAlbum-x86_64-<version>.spk
testdata/           Ảnh mẫu và dữ liệu để chạy thử trên máy
```

Dữ liệu trên NAS nằm ở `/var/packages/StudioAlbum/var`: `studioalbum.db`, `cache/` (thumbnail và preview), `branding/`, `studioalbum.log`. **App không sửa hay xóa file ảnh gốc.** Xóa album chỉ xóa dữ liệu trong app.

## 5. Build và chạy thử trên máy

```powershell
.\build.ps1 -Version 1.0.0 -Build 1     # ra dist\StudioAlbum-x86_64-1.0.0.spk
```

Tạo ảnh mẫu một lần: `cd backend; go run ./tools/gensample ../testdata/shares`. Chạy thử trên Windows: `testdata\run-dev.cmd`, rồi mở `http://localhost:8686`. Ở chế độ dev, `/console/` không cần đăng nhập DSM. Test backend: `cd backend && go test ./...`.

Quên mật khẩu chủ mà không mở được Console thì SSH vào NAS rồi chạy:
`sudo /var/packages/StudioAlbum/target/bin/studioalbum reset-admin -data /var/packages/StudioAlbum/var admin MatKhauMoi`

## 6. Ghi chú kỹ thuật

- **Thumbnail**:
  - Ảnh 480px tạo nền khi album được quét. Nếu thư mục đã được Synology Photos index thì dùng lại `@eaDir/SYNOPHOTO_THUMB_XL.jpg`, nhanh hơn nhiều.
  - Ảnh preview 2048px tạo khi khách mở ảnh, có giới hạn dung lượng (mặc định 20GB, chỉnh trong Console).
  - Ảnh xoay đúng chiều theo EXIF.
- **Bảo mật**:
  - Mật khẩu hash bcrypt. Khóa 15 phút khi đăng nhập sai nhiều lần.
  - Cookie HttpOnly/SameSite, Secure khi chạy qua HTTPS. Header chống CSRF.
  - Mọi đường dẫn file được kiểm tra nằm trong share đã bật. Không theo symlink ra ngoài share.
- **Realtime**: WebSocket tự kết nối lại. Reverse proxy phải bật header WebSocket.
