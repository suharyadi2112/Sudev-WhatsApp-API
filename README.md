# 📱 SUDEVWA - WhatsApp Multi-Device API (Go)

REST API untuk kelola WhatsApp Web Multi-Device pakai Go, Echo, PostgreSQL, dan [whatsmeow](https://github.com/tulir/whatsmeow).

## ✨ Fitur Utama

### 🔐 Authentication & Instance Management
- Multi-instance — kelola banyak nomor WhatsApp sekaligus
- QR Code authentication — generate QR untuk pairing device
- Persistent sessions — session survive restart, tersimpan di PostgreSQL
- Auto-reconnect — instance otomatis reconnect setelah server restart
- **Instance reusability** — instance yang logout bisa scan QR ulang tanpa create instance baru
- Graceful logout — cleanup sempurna (device store + session memory)

### 💬 Messaging
- Kirim pesan teks (**by instance ID** atau **by phone number**)
- Kirim media dari URL / upload file
- Support text, image, video, document
- Validasi nomor tujuan sebelum kirim

### 📲 Device & Presence
- **Presence heartbeat** — status "Aktif sekarang" setiap 5 menit
- Realtime status tracking (`online`, `disconnected`, `logged_out`)

## 🛠️ Status
✅ Multi-instance, QR auth, send text/media (by instance ID & phone number), presence, reusable instance  
🚧 Group messaging, templates, broadcast  
📋 webhooks

## ⚠️ Disclaimer
For educational/research purposes only. Use at your own risk.

## 🏗️ Tech Stack
Go 1.21+ (Echo v4) • PostgreSQL 12+ • [whatsmeow](https://github.com/tulir/whatsmeow)

**Made with by SUDEV**

