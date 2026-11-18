package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloud-shuttle/drover-sqlforge/internal/config"
)

const virtualKeyEnv = "SQLFORGE_AI_VIRTUAL_KEY"

// NewGatewayClient targets an OpenAI-compatible Gateway endpoint.
// virtualKey must be the member's own key. A shared key is never read from
// sqlforge.yml, flags, or the repo.
func NewGatewayClient(endpoint, model, virtualKey string, dimensions map[string]string) (*Client, error) {
	if strings.TrimSpace(virtualKey) == "" {
		return nil, fmt.Errorf("ai provider gateway requires %s, a per-member virtual key (never a shared or agent-facing key)", virtualKeyEnv)
	}
	return &Client{
		Endpoint:   endpoint,
		Model:      model,
		Provider:   "gateway",
		VirtualKey: virtualKey,
		Dimensions: dimensions,
	}, nil
}

// ClientFromConfig builds an Ollama client or a Gateway client.
// Gateway mode reads the virtual key only from getenv(SQLFORGE_AI_VIRTUAL_KEY).
func ClientFromConfig(cfg config.AIConfig, getenv func(string) string, environ []string) (*Client, error) {
	if strings.EqualFold(cfg.Provider, "gateway") {
		key := ""
		if getenv != nil {
			key = getenv(virtualKeyEnv)
		}
		return NewGatewayClient(cfg.Endpoint, cfg.Model, key, DimsFromEnviron(environ))
	}
	return NewClient(cfg.Endpoint, cfg.Model), nil
}

// DimsFromEnviron copies X_BF_DIM_* variables to x-bf-dim-* headers.
// Values that contain quotes, newlines, or semicolons are dropped.
func DimsFromEnviron(environ []string) map[string]string {
	out := map[string]string{}
	for _, pair := range environ {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || !strings.HasPrefix(key, "X_BF_DIM_") {
			continue
		}
		if !safeDim(value) {
			continue
		}
		suffix := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(key, "X_BF_DIM_"), "_", "-"))
		out["x-bf-dim-"+suffix] = value
	}
	return out
}

func safeDim(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_' || r == '.' || r == ':' || r == '-':
		default:
			return false
		}
	}
	return true
}

func (c *Client) explainGateway(sqlContext string) (string, error) {
	if c.VirtualKey == "" {
		return nilString()
	}
	payload := map[string]any{
		"model": c.Model,
		"messages": []map[string]string{{
			"role":    "user",
			"content": fmt.Sprintf("Explain the following SQL model:\n\n%s", sqlContext),
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(c.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-bf-vk", c.VirtualKey)
	req.Header.Set("X-Source", "drover-sqlforge")
	for key, value := range c.Dimensions {
		req.Header.Set(key, value)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gateway status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return gatewayContent(raw)
}

func nilString() (string, error) {
	return "", fmt.Errorf("ai provider gateway requires %s, a per-member virtual key (never a shared or agent-facing key)", virtualKeyEnv)
}

func gatewayContent(raw []byte) (string, error) {
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("gateway response had no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
