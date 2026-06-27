package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuthToken OAuth token
type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	TokenType    string
}

// OAuthManager OAuth token 管理器（支持自动刷新）
type OAuthManager struct {
	mu           sync.Mutex
	token        *OAuthToken
	clientID     string
	clientSecret string
	tokenURL     string
	onRefresh    func(token *OAuthToken) error // 刷新回调
}

// NewOAuthManager 创建 OAuth 管理器
func NewOAuthManager(tokenURL, clientID, clientSecret string) *OAuthManager {
	return &OAuthManager{
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// SetRefreshCallback 设置 token 刷新回调
func (m *OAuthManager) SetRefreshCallback(fn func(token *OAuthToken) error) {
	m.onRefresh = fn
}

// SetToken 设置初始 token
func (m *OAuthManager) SetToken(token *OAuthToken) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
}

// GetToken 获取有效 token（自动刷新）
func (m *OAuthManager) GetToken(ctx context.Context) (*OAuthToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token == nil {
		return nil, fmt.Errorf("no token set")
	}

	// 检查是否过期（提前 60 秒刷新）
	if time.Now().Add(60 * time.Second).After(m.token.ExpiresAt) {
		if m.token.RefreshToken != "" {
			newToken, err := m.refreshToken(ctx, m.token.RefreshToken)
			if err != nil {
				return nil, fmt.Errorf("refresh token: %w", err)
			}
			m.token = newToken
			if m.onRefresh != nil {
				if err := m.onRefresh(newToken); err != nil {
					return nil, fmt.Errorf("refresh callback: %w", err)
				}
			}
		}
	}

	return m.token, nil
}

// refreshToken 使用 refresh_token 向 tokenURL 请求新 token，支持 OAuth 2.0 refresh 标准。
func (m *OAuthManager) refreshToken(ctx context.Context, refreshToken string) (*OAuthToken, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {m.clientID},
		"client_secret": {m.clientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", m.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}

	newRefresh := result.RefreshToken
	if newRefresh == "" {
		newRefresh = refreshToken // 服务端未返回新 refresh_token，保持原值
	}

	return &OAuthToken{
		AccessToken:  result.AccessToken,
		RefreshToken: newRefresh,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
		TokenType:    result.TokenType,
	}, nil
}

// IsExpired 检查 token 是否过期
func (m *OAuthManager) IsExpired() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == nil {
		return true
	}
	return time.Now().After(m.token.ExpiresAt)
}

// NeedsRefresh 检查是否需要刷新（提前 60 秒）
func (m *OAuthManager) NeedsRefresh() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == nil {
		return true
	}
	return time.Now().Add(60 * time.Second).After(m.token.ExpiresAt)
}

// AuthorizationHeader 返回 Authorization header 值
func (m *OAuthManager) AuthorizationHeader(ctx context.Context) (string, error) {
	token, err := m.GetToken(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s", token.TokenType, token.AccessToken), nil
}
