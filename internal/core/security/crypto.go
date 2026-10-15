package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CipherService 提供基于 AES-256-GCM 的敏感字段加解密
type CipherService struct {
	key []byte
}

// NewCipherService 创建加密服务，如果 key 为空则自动从 keyPath 加载或创建
func NewCipherService(keyPath string) (*CipherService, error) {
	var key []byte
	if keyPath != "" {
		if data, err := os.ReadFile(keyPath); err == nil && len(data) == 32 {
			key = data
		} else {
			// 生成全新的 32 字节随机主密钥并安全写入
			key = make([]byte, 32)
			if _, err := io.ReadFull(rand.Reader, key); err != nil {
				return nil, fmt.Errorf("failed to generate random key: %w", err)
			}
			if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
				return nil, fmt.Errorf("failed to create key dir: %w", err)
			}
			if err := os.WriteFile(keyPath, key, 0600); err != nil {
				return nil, fmt.Errorf("failed to save key file: %w", err)
			}
		}
	} else {
		// 默认内存密钥（主要用于测试）
		key = make([]byte, 32)
		io.ReadFull(rand.Reader, key)
	}

	return &CipherService{key: key}, nil
}

// NewCipherFromPassphrase 通过用户主密码派生 32 字节密钥
func NewCipherFromPassphrase(passphrase string, salt []byte) *CipherService {
	h := sha256.New()
	h.Write([]byte(passphrase))
	h.Write(salt)
	key := h.Sum(nil)
	return &CipherService{key: key}
}

// Encrypt 将明文字符串使用 AES-GCM 加密，并返回 Base64 编码的密文
func (c *CipherService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 将 Base64 编码的密文解密为明文字符串
func (c *CipherService) Decrypt(cipherBase64 string) (string, error) {
	if cipherBase64 == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", fmt.Errorf("invalid base64 cipher: %w", err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong key or corrupted data): %w", err)
	}

	return string(plaintext), nil
}
