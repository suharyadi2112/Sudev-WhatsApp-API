# SUDEVWA - WhatsApp Multi-Instance API (Go/Golang)

> **WhatsApp Automation API** | **Multi-Device Management** | **Real-time WebSocket** | **Go + Echo + Whatsmeow**

REST API for **WhatsApp Web automation**, **multi-instance management**, and **real-time messaging** built with **Go (Golang)**, **Echo framework**, and **whatsmeow library**.

## 🔍 Keywords
WhatsApp API, WhatsApp Bot, Multi-instance WhatsApp, WhatsApp Automation, Go WhatsApp, Whatsmeow, WebSocket Real-time, REST API, PostgreSQL, Echo Framework

## ✨ Key Features

### 🔐 Authentication & Instance Management
- Multi-instance — manage multiple WhatsApp numbers simultaneously
- QR Code authentication — generate QR for device pairing
- Persistent sessions — sessions survive restart, stored in PostgreSQL
- Auto-reconnect — instances automatically reconnect after server restart
- **Instance reusability** — logged out instances can scan QR again without creating new instance
- **Instance availability control** — `used` flag for external app integration, `keterangan` for notes/tracking
- Graceful logout — complete cleanup (device store + session memory)
- Circle/group management — organize instances by category

### 💬 Messaging
- Send text messages (**by instance ID** or **by phone number**)
- Send media from URL / file upload
- Support text, image, video, document
- Recipient number validation before sending
- **Human-like typing simulation** — variable typing speed, composing/paused presence, random delays
- **Real-time incoming message listener** — listen to incoming messages via WebSocket per instance

### 🤖 WhatsApp Warming System
- **Automated conversation simulation** — warm up WhatsApp accounts with natural dialog
- **Script-based messaging** — create reusable conversation templates with spintax support
- **Bidirectional communication** — Actor A ↔ Actor B automatic message exchange
- **Simulation mode** — test scripts without sending real messages (dry-run)
- **Real message mode** — send actual WhatsApp messages with typing simulation
- **Auto-pause on errors** — automatically pause rooms when instances disconnect
- **Dynamic variables** — `{TIME_GREETING}`, `{DAY_NAME}`, `{DATE}` for contextual messages
- **Interval control** — randomized delays between messages (min/max seconds)
- **Real-time monitoring** — WebSocket events for message status and script progress
- **Drag-and-drop reordering** — easily rearrange script line sequences

### 🔌 Real-time Features (WebSocket)
- **Global WebSocket** (`/ws`) — monitor QR events, status changes, system events for all instances
- **Instance-specific WebSocket** (`/api/listen/:instanceId`) — listen to incoming messages for specific instance
- **Warming events** — real-time warming message status (SUCCESS/FAILED/FINISHED/PAUSED)
- **Ping-based keep-alive** — connection stays alive with ping every 5 minutes
- **Auto-cleanup** — ghost connections automatically removed after 15 minutes timeout
- Support text messages, extended messages, image/video captions
- **Configurable incoming broadcast** — control incoming message WebSocket broadcast via env

### 📲 Device & Presence
- **Random device identity** — unique OS (Windows/macOS/Linux) + hex ID per instance for privacy
- **Presence heartbeat** — "Active now" status every 5 minutes
- Real-time status tracking (`online`, `disconnected`, `logged_out`)

### API Reference

```bash
https://soqnnmoe17.apidog.io/
```

### Global WebSocket - System Events

```bash
ws://127.0.0.1:{port}/ws
```

**Purpose:** Monitor QR code generation, login/logout events, connection status changes for all instances

**Events received:**
- QR code generated
- Instance connected/disconnected
- Instance status changed
- System-wide notifications

### Instance-Specific WebSocket - Incoming Messages

```bash
ws://localhost:2121/api/listen/:instanceId
```

**Purpose:** Listen to incoming WhatsApp messages for a specific instance only

**Headers:**

```http
Authorization: Bearer {token}
```

**Events received:**

```json
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

### 📡 Incoming Message Webhook (Beta)
- Optional HTTP webhook for incoming WhatsApp messages  
- Configurable per instance via REST API  
- Shared payload format with WebSocket `incoming_message` event  

## ⚙️ Environment Variables

Configure these variables in your `.env` file to customize the application behavior.

### 🌐 Core Configuration
| Variable | Description | Default | Example |
| :--- | :--- | :--- | :--- |
| `DATABASE_URL` | PostgreSQL URL for whatsmeow session storage | - | `postgres://user:pass@localhost:5432/db` |
| `APP_DATABASE_URL` | PostgreSQL URL for application data | - | `postgres://user:pass@localhost:5432/app_db` |
| `JWT_SECRET` | Secret key for JWT authentication | - | `YOUR_JWT_SECRET` |
| `APP_LOGIN_USERNAME` | Username for dashboard/API login | - | `sudevwa` |
| `APP_LOGIN_PASSWORD` | Password for dashboard/API login | - | `5ud3vw4` |
| `PORT` | Server listening port | `2121` | `3000` |
| `BASEURL` | Base URL/Host of the server | - | `127.0.0.1` |
| `CORS_ALLOW_ORIGINS` | Allowed origins for CORS | - | `http://localhost:3000` |

### 🛠️ Features & Logic
| Variable | Description | Default | Example |
| :--- | :--- | :--- | :--- |
| `SUDEVWA_ENABLE_WEBSOCKET_INCOMING_MSG` | Enable incoming message WebSocket broadcast | `false` | `true` |
| `SUDEVWA_ENABLE_WEBHOOK` | Enable global incoming message webhooks | `false` | `true` |
| `SUDEVWA_TYPING_DELAY_MIN` | Minimum typing simulation delay (seconds) | `1` | `2` |
| `SUDEVWA_TYPING_DELAY_MAX` | Maximum typing simulation delay (seconds) | `3` | `5` |
| `ALLOW_9_DIGIT_PHONE_NUMBER` | Allow 9-digit numbers without validation | `false` | `true` |

### 🚦 Rate Limiting
| Variable | Description | Default | Example |
| :--- | :--- | :--- | :--- |
| `RATE_LIMIT_PER_SECOND` | API requests allowed per second | `10` | `20` |
| `RATE_LIMIT_BURST` | Max burst of requests | `10` | `20` |
| `RATE_LIMIT_WINDOW_MINUTES` | Rate limit expiration window | `3` | `5` |

### 📁 File Upload Limits (MB)
| Variable | Description | Default | Example |
| :--- | :--- | :--- | :--- |
| `MAX_FILE_SIZE_IMAGE_MB` | Max image upload size | `5` | `10` |
| `MAX_FILE_SIZE_VIDEO_MB` | Max video upload size | `16` | `32` |
| `MAX_FILE_SIZE_AUDIO_MB` | Max audio upload size | `16` | `32` |
| `MAX_FILE_SIZE_DOCUMENT_MB` | Max document upload size | `100` | `200` |

### 🔥 Warming System
| Variable | Description | Default | Example |
| :--- | :--- | :--- | :--- |
| `WARMING_WORKER_ENABLED` | Enable automated conversation simulation | `false` | `true` |
| `WARMING_WORKER_INTERVAL_SECONDS` | Interval between worker checks | `5` | `10` |
| `WARMING_AUTO_REPLY_ENABLED` | Enable AI/Auto-reply in warming rooms | `false` | `true` |
| `WARMING_AUTO_REPLY_COOLDOWN` | Cooldown between auto-replies (seconds) | `60` | `10` |
| `DEFAULT_REPLY_DELAY_MIN` | Min delay before auto-reply (seconds) | `10` | `5` |
| `DEFAULT_REPLY_DELAY_MAX` | Max delay before auto-reply (seconds) | `60` | `30` |

### 🤖 AI Configuration (Gemini)
| Variable | Description | Default | Example |
| :--- | :--- | :--- | :--- |
| `AI_ENABLED` | Enable AI-powered features | `false` | `true` |
| `AI_DEFAULT_PROVIDER` | AI provider (gemini or openai) | `gemini` | `openai` |
| `GEMINI_API_KEY` | Google Gemini API Key | - | `AIzaSy...` |
| `GEMINI_DEFAULT_MODEL` | Default Gemini model to use | `gemini-1.5-flash` | `gemini-pro` |
| `AI_CONVERSATION_HISTORY_LIMIT` | Number of previous messages for context | `10` | `20` |
| `AI_DEFAULT_TEMPERATURE` | AI response randomness (0.0 to 1.0) | `0.7` | `0.5` |
| `AI_DEFAULT_MAX_TOKENS` | Max tokens for AI response | `150` | `300` |
If this variable is not set, or set to anything other than `true`, webhooks will not be sent.

### Configure Webhook per Instance
For webhook security, SUDEVWA signs every outgoing webhook (when a secret is configured for the instance) using an HMAC:

- Header: `X-SUDEVWA-Signature`
- Algorithm: `HMAC-SHA256`
- Message: raw HTTP request body (bytes)
- Key: the instance-specific `webhook_secret`

```http
POST /api/instances/:instanceId/webhook-setconfig
Authorization: Bearer {token}
Content-Type: application/json
```
Example body:
```json
{
"url": "https://your-app.com/wa-webhook",
"secret": "5513de0882c755985f4bb358e5cf027cb10e48a23a377cf77888e310b74aef21"
}
```
Response : 
```json
{
"instanceId": "instance123",
"webhookUrl": "https://your-app.com/wa-webhook",
"secret": "5513de0882c755985f4bb358e5cf027cb10e48a23a377cf77888e310b74aef21"
}
```
Webhook Payload:
```json
{
"event": "incoming_message",
"timestamp": "2025-12-08T13:57:04.147255Z",
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

![GitHub stars](https://img.shields.io/github/stars/suharyadi2112/Sudev-Whatsapp-Tools?style=social)
![GitHub forks](https://img.shields.io/github/forks/suharyadi2112/Sudev-Whatsapp-Tools?style=social)
![GitHub issues](https://img.shields.io/github/issues/suharyadi2112/Sudev-Whatsapp-Tools)
![License](https://img.shields.io/github/license/suharyadi2112/Sudev-Whatsapp-Tools)
![Go Version](https://img.shields.io/github/go-mod/go-version/suharyadi2112/Sudev-Whatsapp-Tools)


## ⭐ Support This Project

If you find this project useful, please consider:
- ⭐ **Star this repository**
- 🍴 **Fork and contribute**
- 🐛 **Report issues**
- 📢 **Share with your network**

Your support helps maintain and improve this project!


**Made by SUDEV**
