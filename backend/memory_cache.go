package backend

import (
	"context"
	"errors"
	"sync"
	"time"
)

// OTPCache defines the interface for OTP storage operations
type OTPCache interface {
	SetOTP(ctx context.Context, phone string, otp string) error
	GetOTP(ctx context.Context, phone string) (string, error)
	DeleteOTP(ctx context.Context, phone string) error
}

const (
	otpExpiration = 5 * time.Minute
)

// Public errors returned by OTPCache operations
var (
	ErrOTPExpired  = errors.New("OTP expired")
	ErrOTPNotFound = errors.New("OTP not found")
)

// MemoryOTPCache implements OTPCache using in-memory storage
type MemoryOTPCache struct {
	mu    sync.RWMutex
	store map[string]otpEntry
}

type otpEntry struct {
	otp       string
	expiresAt time.Time
}

// NewMemoryOTPCache creates a new MemoryOTPCache instance
func NewMemoryOTPCache() *MemoryOTPCache {
	cache := &MemoryOTPCache{
		store: make(map[string]otpEntry),
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// SetOTP stores the OTP in memory with expiration
func (c *MemoryOTPCache) SetOTP(ctx context.Context, phone string, otp string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[phone] = otpEntry{
		otp:       otp,
		expiresAt: time.Now().Add(otpExpiration),
	}
	return nil
}

// GetOTP retrieves the OTP from memory
func (c *MemoryOTPCache) GetOTP(ctx context.Context, phone string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.store[phone]
	if !exists {
		return "", ErrOTPNotFound
	}

	if time.Now().After(entry.expiresAt) {
		delete(c.store, phone)
		return "", ErrOTPExpired
	}

	return entry.otp, nil
}

// DeleteOTP removes the OTP from memory
func (c *MemoryOTPCache) DeleteOTP(ctx context.Context, phone string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.store, phone)
	return nil
}

// cleanup periodically removes expired OTPs
func (c *MemoryOTPCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for phone, entry := range c.store {
			if now.After(entry.expiresAt) {
				delete(c.store, phone)
			}
		}
		c.mu.Unlock()
	}
}
