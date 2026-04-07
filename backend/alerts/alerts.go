// Package alerts sends notifications via Telegram and Discord.
package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"stableguard-backend/store"
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

	mu       sync.Mutex
	lastSent map[string]time.Time // key → last send time (cooldown)
	cooldown time.Duration

	subscriptions TelegramSubscriptionStore
	updateOffset  int64
}

// Snapshot is a point-in-time copy of alert channel configuration.
type Snapshot struct {
	TelegramToken  string
	TelegramChatID string
	DiscordWebhook string
}

type TelegramSubscriptionStore interface {
	ConfirmedTelegramSubscriptions() ([]store.NotificationSubscription, error)
	TelegramLinkToken(token string) (*store.TelegramLinkToken, error)
	ConfirmTelegramSubscription(authType, userKey, chatID, username string) error
	MarkTelegramLinkTokenUsed(token string) error
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

func (c *Client) WithTelegramSubscriptionStore(store TelegramSubscriptionStore) *Client {
	c.subscriptions = store
	return c
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

	if c.telegramToken != "" {
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
	if c.telegramToken != "" {
		c.sendTelegram(full)
	}
	if c.discordWebhook != "" {
		c.sendDiscord(full)
	}
}

func (c *Client) SendTelegramDirect(chatID string, level Level, msg string) {
	if c.telegramToken == "" || strings.TrimSpace(chatID) == "" {
		return
	}
	emoji := map[Level]string{
		LevelInfo:     "ℹ️",
		LevelWarning:  "⚠️",
		LevelCritical: "🚨",
	}[level]
	full := fmt.Sprintf("%s *StableGuard %s*\n%s", emoji, level, msg)
	c.sendTelegramMessage(chatID, full)
}

// Enabled returns true if at least one channel is configured.
func (c *Client) Enabled() bool {
	return c.telegramToken != "" || c.discordWebhook != ""
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

// Snapshot returns the currently configured alert channels.
func (c *Client) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Snapshot{
		TelegramToken:  c.telegramToken,
		TelegramChatID: c.telegramChatID,
		DiscordWebhook: c.discordWebhook,
	}
}

// ── Telegram ───────────────────────────────────────────────────────────────

func (c *Client) sendTelegram(text string) {
	seen := map[string]struct{}{}
	if chatID := strings.TrimSpace(c.telegramChatID); chatID != "" {
		seen[chatID] = struct{}{}
		c.sendTelegramMessage(chatID, text)
	}
	if c.subscriptions == nil {
		return
	}
	subs, err := c.subscriptions.ConfirmedTelegramSubscriptions()
	if err != nil {
		log.Printf("[alerts/telegram] subscriptions error: %v", err)
		return
	}
	for _, sub := range subs {
		chatID := strings.TrimSpace(sub.TelegramChatID)
		if chatID == "" {
			continue
		}
		if _, ok := seen[chatID]; ok {
			continue
		}
		seen[chatID] = struct{}{}
		c.sendTelegramMessage(chatID, text)
	}
}

func (c *Client) sendTelegramMessage(chatID, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.telegramToken)
	body, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	})
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[alerts/telegram] send error for chat %s: %v", chatID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[alerts/telegram] HTTP %d for chat %s", resp.StatusCode, chatID)
	}
}

type telegramUpdatesResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	Text string        `json:"text"`
	Chat telegramChat  `json:"chat"`
	From *telegramUser `json:"from"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramUser struct {
	Username string `json:"username"`
}

func (c *Client) SyncTelegramUpdates() {
	if c.telegramToken == "" || c.subscriptions == nil {
		return
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=1&offset=%d", c.telegramToken, c.updateOffset+1)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		log.Printf("[alerts/telegram] getUpdates error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[alerts/telegram] getUpdates HTTP %d", resp.StatusCode)
		return
	}

	var payload telegramUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		log.Printf("[alerts/telegram] decode updates error: %v", err)
		return
	}
	for _, update := range payload.Result {
		if update.UpdateID > c.updateOffset {
			c.updateOffset = update.UpdateID
		}
		msg := update.Message
		if msg == nil {
			continue
		}
		token := parseStartToken(msg.Text)
		if token == "" {
			continue
		}
		link, err := c.subscriptions.TelegramLinkToken(token)
		if err != nil || link == nil {
			continue
		}
		if err := c.subscriptions.ConfirmTelegramSubscription(
			link.AuthType,
			link.UserKey,
			fmt.Sprintf("%d", msg.Chat.ID),
			telegramUsername(msg.From),
		); err != nil {
			log.Printf("[alerts/telegram] confirm subscription error: %v", err)
			continue
		}
		if err := c.subscriptions.MarkTelegramLinkTokenUsed(token); err != nil {
			log.Printf("[alerts/telegram] mark token used error: %v", err)
		}
		c.sendTelegramMessage(fmt.Sprintf("%d", msg.Chat.ID), "✅ *StableGuard Telegram connected*\nYou will now receive treasury alerts and execution reports here.")
	}
}

func parseStartToken(text string) string {
	text = strings.TrimSpace(text)
	const prefix = "/start stableguard_"
	if strings.HasPrefix(text, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(text, prefix))
	}
	return ""
}

func telegramUsername(user *telegramUser) string {
	if user == nil {
		return ""
	}
	return strings.TrimSpace(user.Username)
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
