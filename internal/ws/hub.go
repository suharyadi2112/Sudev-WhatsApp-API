package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client merepresentasikan satu koneksi WebSocket ke FE.
type Client struct {
	hub  *Hub
	conn *websocket.Conn

	// Channel untuk mengirim event ke client ini.
	// Goroutine write akan membaca dari sini dan mengirim ke conn.
	send chan WsEvent

	// (Opsional) informasi identitas untuk filtering ke depan,
	// misalnya UserID / TenantID / daftar InstanceID yang di-subscribe.
	// Untuk versi awal bisa dikosongkan dulu.
	// UserID string
}

// Hub menyimpan semua client aktif dan menangani broadcast event.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Register / unregister requests from clients.
	register   chan *Client
	unregister chan *Client

	// Broadcast adalah channel event yang akan dikirim ke semua client.
	broadcast chan WsEvent

	// Mutex kalau nanti butuh akses synchronous ke clients dari luar Run().
	mu sync.RWMutex
}

// NewHub membuat instance Hub baru.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan WsEvent, 256), // buffer kecil untuk mencegah blocking
	}
}

// Run harus dijalankan di goroutine terpisah.
// Loop ini akan:
// - menerima client baru (register)
// - menghapus client yang disconnect (unregister)
// - mengirim event ke semua client (broadcast)
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- event:
					// sukses kirim ke buffer client
				default:
					// kalau buffer penuh, anggap client bermasalah dan putuskan
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register digunakan oleh handler WS saat koneksi baru dibuat.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister dipanggil ketika koneksi WS ditutup.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Publish mengimplementasikan RealtimePublisher.
// Service lain cukup memanggil ini untuk mengirim event ke semua client.
func (h *Hub) Publish(event WsEvent) {
	// Pastikan timestamp terisi kalau belum diset.
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	h.broadcast <- event
}

// RealtimePublisher adalah interface yang akan dipegang oleh service lain
// (whatsapp.go, handler QR) agar tidak tergantung langsung ke Hub.
type RealtimePublisher interface {
	Publish(event WsEvent)
}

// NewClient membuat objek Client baru dari koneksi Gorilla WebSocket.
// Fungsi ini tidak menjalankan goroutine read/write; itu tugas handler WS.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan WsEvent, 256),
	}
}

// WritePump adalah loop yang mengirim event dari channel send ke koneksi WS.
// Biasanya dipanggil sebagai goroutine dari handler /ws.
func (c *Client) WritePump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	for event := range c.send {
		// Encode WsEvent ke JSON.
		payload, err := json.Marshal(event)
		if err != nil {
			log.Printf("ws: failed to marshal event: %v", err)
			continue
		}

		// Set deadline sederhana supaya tidak hang selamanya.
		_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

		if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("ws: failed to write message: %v", err)
			return
		}
	}
}

// ReadPump opsional: loop baca dari client.
// Untuk versi awal, kamu bisa hanya consume dan buang,
// atau pakai untuk menerima perintah subscribe dsb.
// Kalau belum butuh, bisa dibuat minimal / dikosongkan.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512)

	_ = c.conn.SetReadDeadline(time.Now().Add(15 * time.Minute))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(15 * time.Minute))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("ws read error: %v", err)
			break
		}
	}
}
