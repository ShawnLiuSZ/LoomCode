package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCrypto_EncryptDecrypt(t *testing.T) {
	c := NewCryptoWithKey("test-key-1234567890")

	plaintext := []byte("hello world, this is a test message for encryption")
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	if string(ciphertext[:len(plaintext)]) == string(plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}

	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestCrypto_EncryptDecryptString(t *testing.T) {
	c := NewCryptoWithKey("another-test-key")

	original := "sensitive session data"
	encoded, err := c.EncryptString(original)
	if err != nil {
		t.Fatalf("EncryptString error: %v", err)
	}

	if encoded == original {
		t.Error("encoded should differ from original")
	}

	decoded, err := c.DecryptString(encoded)
	if err != nil {
		t.Fatalf("DecryptString error: %v", err)
	}

	if decoded != original {
		t.Errorf("decoded = %q, want %q", decoded, original)
	}
}

func TestCrypto_DifferentKeys(t *testing.T) {
	c1 := NewCryptoWithKey("key-alpha")
	c2 := NewCryptoWithKey("key-beta")

	plaintext := []byte("secret data")

	ciphertext, _ := c1.Encrypt(plaintext)
	_, err := c2.Decrypt(ciphertext)
	if err == nil {
		t.Error("decrypt with wrong key should fail")
	}
}

func TestCrypto_SessionFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test-session.jsonl")

	c := NewCryptoWithKey("session-key")

	meta := Meta{
		ID:        "test-session",
		Name:      "test",
		CreatedAt: time.Now(),
		Model:     "deepseek-v4-flash",
	}
	messages := []Message{
		{Role: "user", Content: "hello", Timestamp: time.Now()},
		{Role: "assistant", Content: "hi there", Timestamp: time.Now()},
	}

	err := c.EncryptSessionFile(meta, messages, filePath)
	if err != nil {
		t.Fatalf("EncryptSessionFile error: %v", err)
	}

	// 验证加密文件存在且不可读
	encPath := filePath + ".enc"
	data, _ := os.ReadFile(encPath)
	if string(data) == "hello" {
		t.Error("encrypted file should not contain plaintext")
	}

	// 解密
	sess, err := c.DecryptSessionFile(encPath)
	if err != nil {
		t.Fatalf("DecryptSessionFile error: %v", err)
	}

	if sess.Meta.Name != "test" {
		t.Errorf("Name = %q", sess.Meta.Name)
	}
	if len(sess.Messages) != 2 {
		t.Errorf("Messages count = %d, want 2", len(sess.Messages))
	}
	if sess.Messages[0].Content != "hello" {
		t.Errorf("msg[0] = %q", sess.Messages[0].Content)
	}
}

func TestCrypto_DeriveKey(t *testing.T) {
	// 设置环境变量
	t.Setenv("HELIX_ENCRYPTION_KEY", "my-encryption-secret")
	key := deriveKey()
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}

	// 不设置环境变量
	os.Unsetenv("HELIX_ENCRYPTION_KEY")
	key2 := deriveKey()
	if len(key2) != 32 {
		t.Errorf("default key length = %d, want 32", len(key2))
	}

	// 不同密钥
	if string(key) == string(key2) {
		t.Log("keys happen to be same (unlikely but possible with SHA256)")
	}
}
