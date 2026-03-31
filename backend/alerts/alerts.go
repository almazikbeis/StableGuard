// Package alerts sends notifications via Telegram and Discord.
package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Level classifies alert severity.
type Level string

const (
	LevelInfo     Level = "INFO"
	LevelWarning  Level = "WARNING"
	LevelCritical Level = "CRITICAL"
)

// Client sends alerts to Telegram and/or Discord.
type Client struct {
	telegramToken  string
	telegramChatID string
	discordWebhook string
	httpClient     *http.Client

	mu        sync.Mutex
	lastSent  map[string]time.Time // key → last send time (cooldown)
	cooldown  time.Duration
}

// New creates an alert client. Empty strings = that channel disabled.
func New(telegramToken, telegramChatID, discordWebhook string, cooldown time.Duration) *Client {
	return &Client{
		telegramToken:  telegramToken,
		telegramChatID: telegramChatID,
		discordWebhook: discordWebhook,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		lastSent:       make(map[string]time.Time),
		cooldown:       cooldown,
	}
}

// Send sends an alert with deduplication via cooldown key.
// key should be a short identifier like "depeg_usdt" or "risk_critical".
func (c *Client) Send(key string, level Level, msg string) {
	c.mu.Lock()
	if last, ok := c.lastSent[key]; ok && time.Since(last) < c.cooldown {
		c.mu.Unlock()
		return // still in cooldown
	}
	c.lastSent[key] = time.Now()
	c.mu.Unlock()

	emoji := map[Level]string{
		LevelInfo:     "ℹ️",
		LevelWarning:  "⚠️",
		LevelCritical: "🚨",
	}[level]

	full := fmt.Sprintf("%s *StableGuard %s*\n%s", emoji, level, msg)
	log.Printf("[alert/%s] %s: %s", level, key, msg)

	if c.telegramToken != "" && c.telegramChatID != "" {
		go c.sendTelegram(full)
	}
	if c.discordWebhook != "" {
		go c.sendDiscord(full)
	}
}

// SendForce sends immediately, bypassing cooldown.
func (c *Client) SendForce(level Level, msg string) {
	emoji := map[Level]string{
		LevelInfo:     "ℹ️",
		LevelWarning:  "⚠️",
		LevelCritical: "🚨",
	}[level]
	full := fmt.Sprintf("%s *StableGuard %s*\n%s", emoji, level, msg)
	log.Printf("[alert/force/%s] %s", level, msg)
	if c.telegramToken != "" && c.telegramChatID != "" {
		c.sendTelegram(full)
	}
	if c.discordWebhook != "" {
		c.sendDiscord(full)
	}
}

// Enabled returns true if at least one channel is configured.
func (c *Client) Enabled() bool {
	return (c.telegramToken != "" && c.telegramChatID != "") || c.discordWebhook != ""
}

// UpdateTelegram updates Telegram credentials at runtime.
func (c *Client) UpdateTelegram(token, chatID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.telegramToken = token
	c.telegramChatID = chatID
}

// UpdateDiscord updates Discord webhook at runtime.
func (c *Client) UpdateDiscord(webhook string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.discordWebhook = webhook
}

// ── Telegram ───────────────────────────────────────────────────────────────

func (c *Client) sendTelegram(text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.telegramToken)
	body, _ := json.Marshal(map[string]interface{}{
		"chat_id":    c.telegramChatID,
		"text":       text,
		"parse_mode": "Markdown",
	})
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[alerts/telegram] error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[alerts/telegram] HTTP %d", resp.StatusCode)
	}
}

// ── Discord ────────────────────────────────────────────────────────────────

func (c *Client) sendDiscord(text string) {
	body, _ := json.Marshal(map[string]string{"content": text})
	resp, err := c.httpClient.Post(c.discordWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[alerts/discord] error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		log.Printf("[alerts/discord] HTTP %d", resp.StatusCode)
	}
}
