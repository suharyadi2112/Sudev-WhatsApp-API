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

### API Reference

```
https://soqnnmoe17.apidog.io/
```

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

### 📡 Incoming Message Webhook (Beta)
- Optional HTTP webhook for incoming WhatsApp messages  
- Configurable per instance via REST API  
- Shared payload format with WebSocket `incoming_message` event  

### Enable via ENV
```
SUDEVWA_ENABLE_WEBSOCKET=true
SUDEVWA_ENABLE_WEBHOOK=true
```
If this variable is not set, or set to anything other than `true`, webhooks will not be sent.

### Configure Webhook per Instance
For webhook security, SUDEVWA signs every outgoing webhook (when a secret is configured for the instance) using an HMAC:

- Header: `X-SUDEVWA-Signature`
- Algorithm: `HMAC-SHA256`
- Message: raw HTTP request body (bytes)
- Key: the instance-specific `webhook_secret`

```
POST /api/instances/:instanceId/webhook-setconfig
Authorization: Bearer {token}
Content-Type: application/json
```
Example body:
```
{
"url": "https://your-app.com/wa-webhook"
"secret": "5513de0882c755985f4bb358e5cf027cb10e48a23a377cf77888e310b74aef21" //optional autogenerate if none
}
```
Response : 
```
{
"instanceId": "instance123",
"webhookUrl": "https://your-app.com/wa-webhook",
"secret": "5513de0882c755985f4bb358e5cf027cb10e48a23a377cf77888e310b74aef21"
}
```
Webhook Payload
```
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
