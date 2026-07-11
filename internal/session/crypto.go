package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Crypto 会话加密器（AES-256-GCM）
type Crypto struct {
	key []byte
}

// NewCrypto 创建加密器
// 密钥来源优先级：HELIX_ENCRYPTION_KEY 环境变量 > 默认派生密钥
func NewCrypto() *Crypto {
	key := deriveKey()
	return &Crypto{key: key}
}

// NewCryptoWithKey 使用指定密钥创建
func NewCryptoWithKey(key string) *Crypto {
	hash := sha256.Sum256([]byte(key))
	return &Crypto{key: hash[:]}
}

// deriveKey 派生加密密钥
// 注意：此模块当前未被生产代码调用，会话实际明文存储。
// 接线前必须改为：随机密钥 + Argon2id/scrypt + 随机 salt。
func deriveKey() []byte {
	// 从环境变量读取（推荐方式）
	if envKey := os.Getenv("HELIX_ENCRYPTION_KEY"); envKey != "" {
		hash := sha256.Sum256([]byte(envKey))
		return hash[:]
	}

	// 未设置环境变量时生成随机密钥（每次启动不同，会话无法跨重启解密）
	// 这是安全的默认行为，比用公开信息（hostname/username）好
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		// 极端回退：用时间戳 + PID（不安全，但比 panic 好）
		seed := fmt.Sprintf("helix-session-%d-%d", time.Now().UnixNano(), os.Getpid())
		hash := sha256.Sum256([]byte(seed))
		return hash[:]
	}
	return key
}

// Encrypt 加密数据
func (c *Crypto) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// nonce + ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 解密数据
func (c *Crypto) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptString 加密字符串
func (c *Crypto) EncryptString(plaintext string) (string, error) {
	data, err := c.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// DecryptString 解密字符串
func (c *Crypto) DecryptString(encoded string) (string, error) {
	data, err := hex.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("hex decode: %w", err)
	}
	plaintext, err := c.Decrypt(data)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptSessionFile 加密会话文件
func (c *Crypto) EncryptSessionFile(meta Meta, messages []Message, filePath string) error {
	// 序列化
	type sessionData struct {
		Meta     Meta      `json:"meta"`
		Messages []Message `json:"messages"`
	}
	data := sessionData{Meta: meta, Messages: messages}

	plaintext, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	encrypted, err := c.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// 写入文件（hex 编码）
	encoded := hex.EncodeToString(encrypted)
	return os.WriteFile(filePath+".enc", []byte(encoded), 0600)
}

// DecryptSessionFile 解密会话文件
func (c *Crypto) DecryptSessionFile(filePath string) (*Session, error) {
	// H14 修复：EncryptSessionFile 写入的是 filePath+".enc"，此处必须读同一路径，
	// 否则二者不对称，永远解密不到正确的密文文件。
	encPath := filePath
	if !strings.HasSuffix(encPath, ".enc") {
		encPath = filePath + ".enc"
	}
	encoded, err := os.ReadFile(encPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	data, err := hex.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}

	plaintext, err := c.Decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	type sessionData struct {
		Meta     Meta      `json:"meta"`
		Messages []Message `json:"messages"`
	}
	var sd sessionData
	if err := json.Unmarshal(plaintext, &sd); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return &Session{
		Meta:     sd.Meta,
		Messages: sd.Messages,
	}, nil
}
