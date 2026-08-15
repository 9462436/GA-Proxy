package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

const Version = "1.0.0"

// HashKey 纯 Go 实现的安全握手签名（替代原 Plan 9 汇编 Obfuscate）
// 客户端可对比 signature 是否与服务端一致
func HashKey(challenge uint64, secret string) string {
	key := uint64(0)
	for _, ch := range secret {
		key += uint64(ch)
	}
	return fmt.Sprintf("%x", challenge^key)
}

func main() {
	_ = godotenv.Load()

	app := fiber.New(fiber.Config{
		AppName: "groq-proxy-v" + Version,
	})

	app.Use(logger.New())
	app.Use(cors.New())

	maxReqs := 20
	fmt.Sscanf(os.Getenv("RATE_LIMIT_MAX"), "%d", &maxReqs)

	app.Use(limiter.New(limiter.Config{
		Max:        maxReqs,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"error":       "Rate limit exceeded",
				"retry_after": "1 minute",
			})
		},
	}))

	app.Use(func(c *fiber.Ctx) error {
		secret := os.Getenv("PROXY_SECRET")
		if secret == "" {
			return c.Next()
		}
		if c.Path() == "/" || c.Path() == "/handshake" || c.Path() == "/version" {
			return c.Next()
		}
		if c.Get("X-GA-Secret") != secret {
			return c.Status(403).JSON(fiber.Map{"error": "Unauthorized: Invalid GA Secret"})
		}
		return c.Next()
	})

	app.Get("/version", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"version": Version})
	})

	app.Get("/handshake", func(c *fiber.Ctx) error {
		challenge := c.QueryInt("challenge", 0)
		secret := os.Getenv("PROXY_SECRET")
		if secret == "" {
			secret = "GA-DEFAULT-SECRET"
		}
		var services []string
		if os.Getenv("GROQ_API_KEY") != "" {
			services = append(services, "groq")
		}
		if os.Getenv("OPENROUTER_API_KEY") != "" {
			services = append(services, "openrouter")
		}
		if os.Getenv("CEREBRAS_API_KEY") != "" {
			services = append(services, "cerebras")
		}
		return c.JSON(fiber.Map{
			"status":    "ready",
			"version":   Version,
			"signature": HashKey(uint64(challenge), secret),
			"services":  services,
		})
	})

	// 直接代理：/groq/* → https://api.groq.com/openai/*
	app.All("/groq/*", func(c *fiber.Ctx) error {
		apiKey := os.Getenv("GROQ_API_KEY")
		if apiKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "GROQ_API_KEY not configured"})
		}
		// 注意：Fiber 的 Proxy 不支持 SSE 流式，这里用手写透传
		path := c.Path()[len("/groq"):]
		target := "https://api.groq.com/openai" + path + "?" + string(c.Request().URI().QueryString())
		return proxyRequest(c, target, apiKey)
	})

	app.All("/openrouter/*", func(c *fiber.Ctx) error {
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "OPENROUTER_API_KEY not configured"})
		}
		path := c.Path()[len("/openrouter"):]
		target := "https://openrouter.ai/api" + path + "?" + string(c.Request().URI().QueryString())
		return proxyRequest(c, target, apiKey)
	})

	app.All("/cerebras/*", func(c *fiber.Ctx) error {
		apiKey := os.Getenv("CEREBRAS_API_KEY")
		if apiKey == "" {
			return c.Status(500).JSON(fiber.Map{"error": "CEREBRAS_API_KEY not configured"})
		}
		path := c.Path()[len("/cerebras"):]
		target := "https://api.cerebras.ai" + path + "?" + string(c.Request().URI().QueryString())
		return proxyRequest(c, target, apiKey)
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":   "online",
			"name":     "groq-proxy",
			"version":  Version,
			"services": enabledServices(),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.Printf("groq-proxy %s starting on port %s", Version, port)
	log.Fatal(app.Listen(":" + port))
}

func enabledServices() []string {
	var s []string
	if os.Getenv("GROQ_API_KEY") != "" {
		s = append(s, "groq")
	}
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		s = append(s, "openrouter")
	}
	if os.Getenv("CEREBRAS_API_KEY") != "" {
		s = append(s, "cerebras")
	}
	return s
}
