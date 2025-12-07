# 📱 SUDEVWA - WhatsApp Multi-Device API (Go)

REST API for managing WhatsApp Web Multi-Device using Go, Echo, PostgreSQL, and [whatsmeow](https://github.com/tulir/whatsmeow).

## ✨ Key Features

### 🔐 Authentication & Instance Management
- Multi-instance — manage multiple WhatsApp numbers simultaneously
- QR Code authentication — generate QR for device pairing
- Persistent sessions — sessions survive restart, stored in PostgreSQL
- Auto-reconnect — instances automatically reconnect after server restart
- **Instance reusability** — logged out instances can scan QR again without creating new instance
- Graceful logout — complete cleanup (device store + session memory)

### 💬 Messaging
- Send text messages (**by instance ID** or **by phone number**)
- Send media from URL / file upload
- Support text, image, video, document
- Recipient number validation before sending
- **Real-time incoming message listener** — listen to incoming messages via WebSocket per instance

### 🔌 Real-time Features (WebSocket)
- **Global WebSocket** (`/ws`) — monitor QR events, status changes, system events for all instances
- **Instance-specific WebSocket** (`/api/listen/:instanceId`) — listen to incoming messages for specific instance
- **Ping-based keep-alive** — connection stays alive with ping every 5 minutes
- **Auto-cleanup** — ghost connections automatically removed after 15 minutes timeout
- Support text messages, extended messages, image/video captions

### 📲 Device & Presence
- **Custom device name** — appears as "SUDEVWA Beta" in Linked Devices
- **Presence heartbeat** — "Active now" status every 5 minutes
- Real-time status tracking (`online`, `disconnected`, `logged_out`)

### Global WebSocket - System Events

```
ws://127.0.0.1:{port}/ws
```

**Purpose:** Monitor QR code generation, login/logout events, connection status changes for all instances

**Events received:**
- QR code generated
- Instance connected/disconnected
- Instance status changed
- System-wide notifications

### Instance-Specific WebSocket - Incoming Messages

```
ws://localhost:2121/api/listen/:instanceId
```

**Purpose:** Listen to incoming WhatsApp messages for a specific instance only

**Headers:**

```
Authorization: Bearer {token}
```

**Events received:**

```
{
  "event": "incoming_message",
  "timestamp": "2025-12-07T23:22:00Z",
  "data": {
    "instance_id": "instance123",
    "from": "6281234567890@s.whatsapp.net",
    "from_me": false,
    "message": "Hello World",
    "timestamp": 1733587980,
    "is_group": false,
    "message_id": "3EB0ABC123DEF456",
    "push_name": "John Doe"
  }
}
```

## ⚠️ Disclaimer
For educational/research purposes only. Use at your own risk.

## 🏗️ Tech Stack
Go 1.21+ (Echo v4) • PostgreSQL 12+ • [whatsmeow](https://github.com/tulir/whatsmeow) • Gorilla WebSocket

**Made by SUDEV**
