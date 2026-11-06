package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if len(jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be set and at least 32 characters")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  os.Getenv("DATABASE_URL"),
		PreferSimpleProtocol: true,
	}), &gorm.Config{})
	if err != nil {
		log.Fatal("db connect: ", err)
	}
	if err := db.AutoMigrate(&User{}, &Submission{}); err != nil {
		log.Fatal("migrate: ", err)
	}

	if err := setupGoogle(context.Background()); err != nil {
		log.Fatal("google clients: ", err)
	}

	cfg := fiber.Config{
		BodyLimit:   64 * 1024,         // 64 KB max request (limits code size)
		ReadTimeout: 10 * time.Second,  // max time to receive one request
		IdleTimeout: 620 * time.Second, // keep idle connections longer than a load balancer does
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			msg := "internal error"
			var e *fiber.Error
			if errors.As(err, &e) { // errors we made with fiber.NewError
				code, msg = e.Code, e.Message
			} else {
				log.Println("error:", err) // log the real error, hide it from the client
			}
			return c.Status(code).JSON(fiber.Map{"error": msg})
		},
	}

	// Behind a load balancer (Kubernetes) every request seems to come from the
	// load balancer. TRUSTED_PROXIES lists the addresses we believe, so that
	// c.IP() (used by the rate limiter) is the real client. Unset = trust nobody.
	if list := os.Getenv("TRUSTED_PROXIES"); list != "" {
		cfg.TrustProxy = true
		cfg.ProxyHeader = fiber.HeaderXForwardedFor
		cfg.EnableIPValidation = true
		cfg.TrustProxyConfig = fiber.TrustProxyConfig{Proxies: strings.Split(list, ",")}
	}

	app := fiber.New(cfg)
	app.Use(recover.New(), logger.New())

	app.Get("/health", func(c fiber.Ctx) error { return c.SendString("ok") })

	authLimit := limiter.New(limiter.Config{Max: 20}) // 20 requests/min per IP
	app.Post("/auth/register", authLimit, register(db))
	app.Post("/auth/login", authLimit, login(db))

	app.Get("/me", authRequired, me(db))
	app.Post("/run", authRequired, runHandler(db))
	app.Get("/submissions", authRequired, listSubmissions(db))

	port := os.Getenv("PORT") // Cloud Run injects PORT
	if port == "" {
		port = "8080"
	}

	// Start the server in the background, then wait for either a start-up
	// error or a stop signal (Kubernetes and Cloud Run send SIGTERM).
	stop, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	listenErr := make(chan error, 1)
	go func() { listenErr <- app.Listen(":" + port) }()

	select {
	case err := <-listenErr:
		log.Fatal("listen: ", err)
	case <-stop.Done():
		log.Println("stop signal received, finishing running requests...")
	}

	// Let requests that are waiting for a runner finish (one takes up to 60 s).
	if err := app.ShutdownWithTimeout(65 * time.Second); err != nil {
		log.Println("shutdown:", err)
	}
	log.Println("bye")
}
