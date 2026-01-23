package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gowa-yourself/config"
	"gowa-yourself/database"
	"gowa-yourself/internal/handler"
	warmingHandler "gowa-yourself/internal/handler/warming"
	"gowa-yourself/internal/helper"
	customMiddleware "gowa-yourself/internal/middleware"
	"gowa-yourself/internal/service"
	"gowa-yourself/internal/worker"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"gowa-yourself/internal/ws"
)

func main() {

	// Load .env (abaikan error kalau file tidak ada, misal di production)
	_ = godotenv.Load()

	//database whatsmeow
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	database.InitWhatsmeow(dbURL)

	//database custom
	appDbURL := os.Getenv("APP_DATABASE_URL")
	if appDbURL == "" {
		log.Fatal("APP_DATABASE_URL is not set")
	}
	database.InitAppDB(appDbURL)

	// Initialize separate Outbox DB if set
	outboxDbURL := os.Getenv("OUTBOX_DATABASE_URL")
	database.InitOutboxDB(outboxDbURL)

	// feature flags (WEBHOOK & WEBSOCKET)
	wsEnv := strings.ToLower(os.Getenv("SUDEVWA_ENABLE_WEBSOCKET_INCOMING_MSG"))
	webhookEnv := strings.ToLower(os.Getenv("SUDEVWA_ENABLE_WEBHOOK"))

	config.EnableWebsocketIncomingMessage = (wsEnv == "true")
	config.EnableWebhook = (webhookEnv == "true")

	autoReplyEnv := os.Getenv("WARMING_AUTO_REPLY_ENABLED")
	config.WarmingAutoReplyEnabled = (autoReplyEnv == "true")

	cooldownStr := os.Getenv("WARMING_AUTO_REPLY_COOLDOWN")
	if cooldownStr != "" {
		if cooldown, err := strconv.Atoi(cooldownStr); err == nil && cooldown > 0 {
			config.WarmingAutoReplyCooldown = cooldown
		} else {
			config.WarmingAutoReplyCooldown = 60 // default 60 seconds
		}
	} else {
		config.WarmingAutoReplyCooldown = 60 // default 60 seconds
	}

	// AI Configuration
	config.AIEnabled = os.Getenv("AI_ENABLED") == "true"
	config.AIDefaultProvider = os.Getenv("AI_DEFAULT_PROVIDER")
	if config.AIDefaultProvider == "" {
		config.AIDefaultProvider = "gemini" // default to free Gemini
	}

	config.GeminiAPIKey = os.Getenv("GEMINI_API_KEY")
	config.GeminiDefaultModel = os.Getenv("GEMINI_DEFAULT_MODEL")
	if config.GeminiDefaultModel == "" {
		config.GeminiDefaultModel = "gemini-1.5-flash"
	}

	if histLimit := os.Getenv("AI_CONVERSATION_HISTORY_LIMIT"); histLimit != "" {
		if limit, err := strconv.Atoi(histLimit); err == nil && limit > 0 {
			config.AIConversationHistoryLimit = limit
		} else {
			config.AIConversationHistoryLimit = 10
		}
	} else {
		config.AIConversationHistoryLimit = 10
	}

	if temp := os.Getenv("AI_DEFAULT_TEMPERATURE"); temp != "" {
		if t, err := strconv.ParseFloat(temp, 64); err == nil && t >= 0 && t <= 1 {
			config.AIDefaultTemperature = t
		} else {
			config.AIDefaultTemperature = 0.7
		}
	} else {
		config.AIDefaultTemperature = 0.7
	}

	if maxTokens := os.Getenv("AI_DEFAULT_MAX_TOKENS"); maxTokens != "" {
		if tokens, err := strconv.Atoi(maxTokens); err == nil && tokens > 0 {
			config.AIDefaultMaxTokens = tokens
		} else {
			config.AIDefaultMaxTokens = 150
		}
	} else {
		config.AIDefaultMaxTokens = 150
	}

	log.Printf("feature flags -> websocket_incoming_msg: %v, webhook: %v, warming_auto_reply: %v, ai_enabled: %v",
		config.EnableWebsocketIncomingMessage, config.EnableWebhook, config.WarmingAutoReplyEnabled, config.AIEnabled)

	//jwt secret for new auth system
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("JWT_SECRET is not set")
	}

	// Initialize authentication service (for new user management)
	service.InitAuthConfig(jwtSecret)

	// **************************
	// main proses.
	//***************************

	runCreateSchema := false
	if len(os.Args) > 1 && os.Args[1] == "--createschema" {
		runCreateSchema = true
	}
	if runCreateSchema { // buat/ensure schema dulu
		helper.InitCustomSchema()
	}

	// Load all existing devices from database
	log.Println("Loading existing devices...")
	err := service.LoadAllDevices()
	if err != nil {
		log.Printf("Warning: Failed to load devices: %v", err)
	}

	// Inisialisasi WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	service.Realtime = hub

	// Setup Echo
	e := echo.New()
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	//env allow ip
	originsEnv := os.Getenv("CORS_ALLOW_ORIGINS")
	if originsEnv == "" {
		log.Println("CORS_ALLOW_ORIGINS is not set")
	}
	allowOrigins := strings.Split(originsEnv, ",")
	for i, o := range allowOrigins {
		allowOrigins[i] = strings.TrimSpace(o)
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: allowOrigins,
		AllowMethods: []string{
			echo.GET,
			echo.POST,
			echo.PUT,
			echo.PATCH,
			echo.DELETE,
			echo.OPTIONS,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderXRequestedWith,
			echo.HeaderAuthorization,
		},
		AllowCredentials: true,
	}))
	e.OPTIONS("/*", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	// Rate limiter configuration from env
	rateLimit := helper.GetEnvAsInt("RATE_LIMIT_PER_SECOND", 10)
	rateBurst := helper.GetEnvAsInt("RATE_LIMIT_BURST", 10)
	rateWindow := helper.GetEnvAsInt("RATE_LIMIT_WINDOW_MINUTES", 3)

	e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(rateLimit),
				Burst:     rateBurst,
				ExpiresIn: time.Duration(rateWindow) * time.Minute,
			},
		),
	}))

	// =====================================================
	// PUBLIC ROUTES (No authentication required)
	// =====================================================

	// New user authentication endpoints
	e.POST("/register", handler.Register)
	e.POST("/login", handler.LoginUser)
	e.POST("/refresh", handler.RefreshToken)

	// Static file serving for uploaded files
	e.Static("/uploads", "./uploads")

	// WebSocket and health check
	e.GET("/ws", handler.WebSocketHandler(hub)) //listen socket gorilla
	e.GET("/", func(c echo.Context) error {     // Health check
		return c.JSON(200, map[string]interface{}{
			"success": true,
			"message": "WhatsApp API is running",
			"version": "1.0.0",
		})
	})

	// Daftar group route yang butuh JWT
	api := e.Group("/api", customMiddleware.JWTAuthMiddleware())

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		message := "Internal Server Error"

		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			message = fmt.Sprintf("%v", he.Message)
		}
		// Custom response format
		response := map[string]interface{}{
			"success": false,
			"error":   message,
		}
		// Custom message untuk error tertentu
		switch code {
		case http.StatusUnauthorized:
			response["message"] = "Authentication required. Please login first."
		case http.StatusMethodNotAllowed:
			response["message"] = "Method not allowed for this endpoint"
		case http.StatusNotFound:
			response["message"] = "Endpoint not found"
		}

		c.JSON(code, response)
	}

	// =====================================================
	// USER PROFILE ROUTES (JWT required)
	// =====================================================
	api.GET("/me", handler.GetCurrentUser)
	api.PUT("/me", handler.UpdateCurrentUser)
	api.PUT("/me/password", handler.ChangePassword)
	api.POST("/logout", handler.LogoutUser)

	// File upload
	api.POST("/me/avatar", handler.UploadAvatar)

	// =====================================================
	// SYSTEM IDENTITY ROUTES (Admin Only)
	// =====================================================
	api.GET("/system/identity", handler.GetSystemIdentityHandler)                                 // Publicly accessible via API token
	api.POST("/system/identity", handler.UpdateSystemIdentityFull, customMiddleware.RequireAdmin) // Unified: Text + Logos (Admin Only)

	// =====================================================
	// WHATSAPP INSTANCE ROUTES (JWT required)
	// =====================================================

	// Routes
	api.POST("/login", handler.Login)
	api.GET("/qr/:instanceId", handler.GetQR, customMiddleware.RequireInstanceAccess())
	api.GET("/status/:instanceId", handler.GetStatus, customMiddleware.RequireInstanceAccess())
	api.POST("/logout/:instanceId", handler.Logout, customMiddleware.RequireInstanceAccess())
	api.DELETE("/instances/:instanceId", handler.DeleteInstance, customMiddleware.RequireInstanceAccess())
	api.DELETE("/qr-cancel/:instanceId", handler.CancelQR, customMiddleware.RequireInstanceAccess())

	// Get all instances (requires authentication, filtered by user role)
	api.GET("/instances", handler.GetAllInstances) // JWT already applied to 'api' group
	// update instance fields (used, keterangan)
	api.PATCH("/instances/:instanceId", handler.UpdateInstanceFields, customMiddleware.RequireInstanceAccess())

	// Message routes by instance id
	api.POST("/send/:instanceId", handler.SendMessage, customMiddleware.RequireInstanceAccess())
	api.POST("/check/:instanceId", handler.CheckNumber, customMiddleware.RequireInstanceAccess())

	// Contact routes
	api.GET("/contacts/:instanceId", handler.GetContactList, customMiddleware.RequireInstanceAccess())
	api.GET("/contacts/:instanceId/export", handler.ExportContacts, customMiddleware.RequireInstanceAccess())
	api.GET("/contacts/:instanceId/:jid", handler.GetContactDetail, customMiddleware.RequireInstanceAccess())
	api.GET("/contacts/:instanceId/:jid/mutual-groups", handler.GetMutualGroups, customMiddleware.RequireInstanceAccess())

	// Media routes by instance id
	api.POST("/send/:instanceId/media", handler.SendMediaFile, customMiddleware.RequireInstanceAccess())
	api.POST("/send/:instanceId/media-url", handler.SendMediaURL, customMiddleware.RequireInstanceAccess())

	//Message by phone number (requires phone number access)
	api.POST("/by-number/:phoneNumber", handler.SendMessageByNumber, customMiddleware.RequirePhoneNumberAccess())
	api.POST("/by-number/:phoneNumber/media-url", handler.SendMediaURLByNumber, customMiddleware.RequirePhoneNumberAccess())
	api.POST("/by-number/:phoneNumber/media-file", handler.SendMediaFileByNumber, customMiddleware.RequirePhoneNumberAccess())

	// Group routes
	api.GET("/groups/:instanceId", handler.GetGroups, customMiddleware.RequireInstanceAccess())
	api.POST("/send-group/:instanceId", handler.SendGroupMessage, customMiddleware.RequireInstanceAccess())
	api.POST("/send-group/:instanceId/media", handler.SendGroupMedia, customMiddleware.RequireInstanceAccess())
	api.POST("/send-group/:instanceId/media-url", handler.SendGroupMediaURL, customMiddleware.RequireInstanceAccess())

	//Group by phone number (requires phone number access)
	api.GET("/groups/by-number/:phoneNumber", handler.GetGroupsByNumber, customMiddleware.RequirePhoneNumberAccess())
	api.POST("/send-group/by-number/:phoneNumber", handler.SendGroupMessageByNumber, customMiddleware.RequirePhoneNumberAccess())
	api.POST("/send-group/by-number/:phoneNumber/media", handler.SendGroupMediaByNumber, customMiddleware.RequirePhoneNumberAccess())
	api.POST("/send-group/by-number/:phoneNumber/media-url", handler.SendGroupMediaURLByNumber, customMiddleware.RequirePhoneNumberAccess())

	//get info akun
	api.GET("/info-device/:instanceId", handler.GetDeviceInfo, customMiddleware.RequireInstanceAccess())

	//----------------------------
	// WEBSOCKET DAN WEBHOOK
	//----------------------------
	//dapatkan pesan masuk, pakai ws
	api.GET("/listen/:instanceId", handler.ListenMessages(hub), customMiddleware.RequireInstanceAccess())
	//webhook
	api.POST("/instances/:instanceId/webhook-setconfig", handler.SetWebhookConfig, customMiddleware.RequireInstanceAccess())

	//----------------------------
	// WORKER BLAST OUTBOX
	//----------------------------
	blastOutbox := api.Group("/blast-outbox")
	blastOutbox.GET("/configs", handler.GetWorkerConfigs)
	blastOutbox.POST("/configs", handler.CreateWorkerConfig)
	blastOutbox.GET("/configs/:id", handler.GetWorkerConfig)
	blastOutbox.PUT("/configs/:id", handler.UpdateWorkerConfig)
	blastOutbox.DELETE("/configs/:id", handler.DeleteWorkerConfig)
	blastOutbox.POST("/configs/:id/toggle", handler.ToggleWorkerConfig)

	// Helper endpoints for frontend
	blastOutbox.GET("/available-circles", handler.GetAvailableCircles)
	blastOutbox.GET("/available-applications", handler.GetAvailableApplications)

	//----------------------------
	// WARMING SYSTEM
	//----------------------------
	warming := api.Group("/warming")
	warming.POST("/scripts", warmingHandler.CreateWarmingScript)
	warming.GET("/scripts", warmingHandler.GetAllWarmingScripts)
	warming.GET("/scripts/:id", warmingHandler.GetWarmingScriptByID)
	warming.PUT("/scripts/:id", warmingHandler.UpdateWarmingScript)
	warming.DELETE("/scripts/:id", warmingHandler.DeleteWarmingScript)

	// Script Lines (Dialog/Naskah)
	// IMPORTANT: Specific routes must come BEFORE parameterized routes to avoid conflicts
	warming.POST("/scripts/:scriptId/lines/generate", warmingHandler.GenerateWarmingScriptLines)
	warming.PUT("/scripts/:scriptId/lines/reorder", warmingHandler.ReorderWarmingScriptLines)
	warming.POST("/scripts/:scriptId/lines", warmingHandler.CreateWarmingScriptLine)
	warming.GET("/scripts/:scriptId/lines", warmingHandler.GetAllWarmingScriptLines)
	warming.GET("/scripts/:scriptId/lines/:id", warmingHandler.GetWarmingScriptLineByID)
	warming.PUT("/scripts/:scriptId/lines/:id", warmingHandler.UpdateWarmingScriptLine)
	warming.DELETE("/scripts/:scriptId/lines/:id", warmingHandler.DeleteWarmingScriptLine)

	// Templates (Manage Conversation Templates)
	warming.POST("/templates", warmingHandler.CreateWarmingTemplate)
	warming.GET("/templates", warmingHandler.GetAllWarmingTemplates)
	warming.GET("/templates/:id", warmingHandler.GetWarmingTemplateByID)
	warming.PUT("/templates/:id", warmingHandler.UpdateWarmingTemplate)
	warming.DELETE("/templates/:id", warmingHandler.DeleteWarmingTemplate)

	// Rooms (Execution Management)
	warming.POST("/rooms", warmingHandler.CreateWarmingRoom)
	warming.GET("/rooms", warmingHandler.GetAllWarmingRooms)
	warming.GET("/rooms/:id", warmingHandler.GetWarmingRoomByID)
	warming.PUT("/rooms/:id", warmingHandler.UpdateWarmingRoom)
	warming.DELETE("/rooms/:id", warmingHandler.DeleteWarmingRoom)
	warming.PATCH("/rooms/:id/status", warmingHandler.UpdateRoomStatus)
	warming.POST("/rooms/:id/restart", warmingHandler.RestartWarmingRoom)

	// Logs (Execution History - Read Only)
	warming.GET("/logs", warmingHandler.GetAllWarmingLogs)
	warming.GET("/logs/:id", warmingHandler.GetWarmingLogByID)

	port := os.Getenv("PORT")
	if port == "" {
		port = "2121" // default aman
	}

	// Start warming worker if enabled
	if os.Getenv("WARMING_WORKER_ENABLED") == "true" {
		log.Println("🚀 Starting Warming Worker...")
		go worker.StartWarmingWorker(hub)
	} else {
		log.Println("⏸️  Warming Worker disabled (set WARMING_WORKER_ENABLED=true to enable)")
	}

	baseURL := os.Getenv("BASEURL")
	if baseURL == "" {
		log.Fatal("BASEURL is not set")
	}

	// log info untuk cek config
	log.Printf("Server starting on port %s, baseURL=%s", port, baseURL)

	// bind ke semua interface, bukan hanya 127.0.0.1
	log.Fatal(e.Start(":" + port))

}
