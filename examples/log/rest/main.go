package main

import (
	"context"
	"time"

	"github.com/ewinjuman/go-lib/v2/constant"
	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var log *logger.Logger

func main() {
	// Inisialisasi logger dengan optimasi performa
	var err error
	log, err = logger.New(logger.Options{
		AppName:        "api-service",
		Environment:    "production",
		Stdout:         true,
		Filename:       "log/api/service.log",
		Write:          true,
		Level:          logger.InfoLevel,
		BufferSize:     1024,                   // Buffer besar untuk throughput tinggi
		FlushInterval:  200 * time.Millisecond, // Flush cepat
		WorkerPoolSize: 4,                      // Sesuaikan dengan jumlah CPU
		MaskingPaths:   []string{"password", "token", "card", "email"},
		RedactionPaths: []string{"secret", "key", "auth"},
	})
	if err != nil {
		panic(err)
	}

	// Penting: Pastikan log buffer di-flush ketika program selesai
	defer log.Shutdown()

	// Setup Fiber app
	app := fiber.New(fiber.Config{
		// Konfigurasi lain
	})

	// Middleware untuk menambahkan trace dan request ID ke setiap request
	app.Use(requestContextMiddleware)

	// API routes
	app.Get("/api/users", getUsers)
	app.Post("/api/users", createUser)
	// Route lainnya...

	// Start server
	app.Listen(":8080")
}

// Middleware untuk menambahkan context ke setiap request
func requestContextMiddleware(c *fiber.Ctx) error {
	// Generate unique IDs for request tracking
	requestID := uuid.New().String()
	traceID := uuid.New().String()

	// Store in Fiber local storage
	c.Locals("requestID", requestID)
	c.Locals("traceID", traceID)
	c.Locals("startTime", time.Now())

	// Set header untuk tracking
	c.Set("X-Request-ID", requestID)
	c.Set("X-Trace-ID", traceID)

	// Jalankan request handler
	err := c.Next()

	// Log request completion setelah handler selesai (non-blocking karena async)
	ctx := createRequestContext(c)
	duration := time.Since(c.Locals("startTime").(time.Time))

	// Log request dan response (asinkron, tidak menambah response time)
	log.Info(ctx, "API Request Completed",
		logger.String("method", c.Method()),
		logger.String("path", c.Path()),
		logger.Int("status", c.Response().StatusCode()),
		logger.Duration("duration_ms", duration),
		logger.String("ip", c.IP()),
	)

	return err
}

// Helper untuk membuat context dari Fiber context
func createRequestContext(c *fiber.Ctx) context.Context {
	ctx := context.Background()

	// Add request and trace IDs to context
	if requestID, ok := c.Locals("requestID").(string); ok {
		ctx = context.WithValue(ctx, constant.RequestIDKey, requestID)
	}

	if traceID, ok := c.Locals("traceID").(string); ok {
		ctx = context.WithValue(ctx, constant.TraceIDKey, traceID)
	}

	// Tambahkan user ID jika ada autentikasi
	if userID := c.Get("X-User-ID", ""); userID != "" {
		ctx = context.WithValue(ctx, constant.UserIDKey, userID)
	}

	return ctx
}

// Contoh handler API
func getUsers(c *fiber.Ctx) error {
	ctx := createRequestContext(c)
	startTime := time.Now()

	// Log info request (non-blocking karena async)
	log.Debug(ctx, "Fetching users",
		logger.String("query", string(c.Request().URI().QueryString())),
	)

	// Simulasi DB query atau business logic
	time.Sleep(100 * time.Millisecond)

	// Simulasi error (uncomment untuk testing)
	/*
		if rand.Intn(10) == 0 {
			err := errors.New("database connection failed")
			log.Error(ctx, "Failed to fetch users",
				logger.Error(err),
				logger.String("query", string(c.Request().URI().QueryString())),
			)
			return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
		}
	*/

	// Prepare response
	users := []map[string]interface{}{
		{"id": 1, "name": "User 1"},
		{"id": 2, "name": "User 2"},
	}

	// Log completion (non-blocking)
	log.Debug(ctx, "Fetched users successfully",
		logger.Int("count", len(users)),
		logger.Duration("duration", time.Since(startTime)),
	)

	return c.JSON(users)
}

// Contoh handler untuk create user
func createUser(c *fiber.Ctx) error {
	ctx := createRequestContext(c)

	// Parse request body
	var user struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&user); err != nil {
		log.Warn(ctx, "Invalid request body",
			logger.Error(err),
			logger.String("body", string(c.Body())),
		)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Log input (dengan masking otomatis untuk field sensitif)
	log.Info(ctx, "Creating new user",
		logger.String("name", user.Name),
		logger.String("email", user.Email),       // Akan otomatis di-mask
		logger.String("password", user.Password), // Akan otomatis di-mask
	)

	// Simulasi create user
	time.Sleep(200 * time.Millisecond)

	// Return response
	return c.Status(201).JSON(fiber.Map{
		"id":    123,
		"name":  user.Name,
		"email": user.Email,
	})
}

// Contoh penggunaan untuk log error dengan stack trace
func logErrorExample(c *fiber.Ctx) error {
	ctx := createRequestContext(c)

	// Simulasi operasi yang error
	err := someOperationThatFails()
	if err != nil {
		// Error log dengan stack trace otomatis (non-blocking)
		log.Error(ctx, "Operation failed",
			logger.Error(err),
			logger.String("additional_info", "Some context about the operation"),
			logger.Int("retry_count", 3),
		)
		return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
	}

	return c.SendString("Success")
}

func someOperationThatFails() error {
	// Simulasi error
	return fiber.NewError(500, "Database connection failed")
}
