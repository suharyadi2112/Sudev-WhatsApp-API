package model

import (
	"context"

	"go.mau.fi/whatsmeow"
)

type Session struct {
	ID              string
	JID             string
	Client          *whatsmeow.Client
	IsConnected     bool
	HeartbeatCancel context.CancelFunc // ✅ FIX: Untuk stop heartbeat goroutine (exported)
}
