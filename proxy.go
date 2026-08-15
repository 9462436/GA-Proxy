package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// proxyRequest 手写的反向代理 —— 完整支持 SSE 流式
func proxyRequest(c *fiber.Ctx, target, apiKey string) error {
	var body io.Reader
	if len(c.Body()) > 0 {
		body = bytes.NewReader(c.Body())
	}

	req, err := http.NewRequest(c.Method(), target, body)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// 透传除鉴权之外的所有头
	c.Request().Header.VisitAll(func(k, v []byte) {
		key := string(k)
		if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Host") {
			return
		}
		req.Header.Set(key, string(v))
	})
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "upstream error: " + err.Error()})
	}
	defer resp.Body.Close()

	// 透传响应头
	resp.Header.VisitAll(func(k, v []byte) {
		c.Set(string(k), string(v))
	})
	c.Status(resp.StatusCode)

	// SSE 流式 —— 走 Fiber SetBodyStreamWriter
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/event-stream") {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Response().SetBodyStream(resp.Body, -1)
		return nil
	}

	// 普通响应
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Send(raw)
}

var httpClient = &http.Client{}