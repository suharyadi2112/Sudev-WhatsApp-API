package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gowa-yourself/database"
	"gowa-yourself/internal/handler"
	"gowa-yourself/internal/helper"
	"gowa-yourself/internal/service"

	"github.com/joho/godotenv"
	echojwt "github.com/labstack/echo-jwt/v4"
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

	//jwt
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("JWT_SECRET is not set")
	}
	handler.InitJWTKey(jwtSecret)

	//user jwt
	handler.InitLoginConfig()

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

	// Rate limiter: 10 request per detik per IP
	e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(10),  // 10 req / detik
				Burst:     10,              // boleh burst sampai 10
				ExpiresIn: 3 * time.Minute, // window penyimpanan per IP
			},
		),
	}))
	e.POST("/login-jwt", handler.LoginJWT)      // di luar group JWT
	e.GET("/ws", handler.WebSocketHandler(hub)) //listen socket gorilla
	e.GET("/", func(c echo.Context) error {     // Health check
		return c.JSON(200, map[string]interface{}{
			"success": true,
			"message": "WhatsApp API is running",
			"version": "1.0.0",
		})
	})

	// Daftar group route yang butuh JWT
	api := e.Group("/api", echojwt.WithConfig(echojwt.Config{
		SigningKey: handler.JwtKey,
		ErrorHandler: func(c echo.Context, err error) error {
			// Custom response untuk JWT authentication error
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"success": false,
				"error":   "Authentication required",
				"message": "Please provide a valid Bearer token in the Authorization header",
			})
		},
	}))
	api.GET("/validate", handler.ValidateToken)

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

	// Routes
	api.POST("/login", handler.Login)
	api.GET("/qr/:instanceId", handler.GetQR)
	api.GET("/status/:instanceId", handler.GetStatus)
	api.POST("/logout/:instanceId", handler.Logout)
	api.DELETE("/instances/:instanceId", handler.DeleteInstance)
	api.DELETE("/qr-cancel/:instanceId", handler.CancelQR)

	//dapatkan pesan masuk, pakai ws
	api.GET("/listen/:instanceId", handler.ListenMessages(hub))

	// ambil semua instance
	api.GET("/instances", handler.GetAllInstances)

	// Message routes by instance id
	api.POST("/send/:instanceId", handler.SendMessage)
	api.POST("/check/:instanceId", handler.CheckNumber)
	// Media routes by instance id
	api.POST("/send/:instanceId/media", handler.SendMediaFile)
	api.POST("/send/:instanceId/media-url", handler.SendMediaURL)

	//Message by nohp
	api.POST("/by-number/:phoneNumber", handler.SendMessageByNumber)
	api.POST("/by-number/:phoneNumber/media-url", handler.SendMediaURLByNumber)
	api.POST("/by-number/:phoneNumber/media-file", handler.SendMediaFileByNumber)

	// Group routes
	api.GET("/groups/:instanceId", handler.GetGroups)
	api.POST("/send-group/:instanceId", handler.SendGroupMessage)
	api.POST("/send-group/:instanceId/media", handler.SendGroupMedia)
	api.POST("/send-group/:instanceId/media-url", handler.SendGroupMediaURL)

	//Group by no hp
	api.GET("/groups/by-number/:phoneNumber", handler.GetGroupsByNumber)
	api.POST("/send-group/by-number/:phoneNumber", handler.SendGroupMessageByNumber)
	api.POST("/send-group/by-number/:phoneNumber/media", handler.SendGroupMediaByNumber)
	api.POST("/send-group/by-number/:phoneNumber/media-url", handler.SendGroupMediaURLByNumber)

	//get info akun
	api.GET("/info-device/:instanceId", handler.GetDeviceInfo)

	port := os.Getenv("PORT")
	if port == "" {
		port = "2121" // default aman
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
