package menus

import (
	"crypto/sha256"
	"encoding/base64"
	"sync"
)

// Telegram rejects any inline button whose callback_data exceeds 64 bytes, and
// it rejects the *whole keyboard* with BUTTON_DATA_INVALID rather than just the
// offending button — so one long pod name makes an entire resource list fail to
// render. Real names blow the limit easily:
//
//	menu:resource:view:pods:default:sample-service-api-5b7d9f4c82-klmno   -> 65 bytes
//	menu:resource:view:configmaps:kube-system:extension-apiserver-...   -> 76 bytes
//
// TokenStore keeps the full callback string server-side and hands out a short
// opaque key, so the button carries "menu:t:AbCdEfGh" (16 bytes) regardless of
// how long the underlying resource name is.
type TokenStore struct {
	mu     sync.RWMutex
	byKey  map[string]string
	byData map[string]string
	order  []string
	limit  int
}

// tokenPrefix marks callback data that must be resolved through the store.
const tokenPrefix = "menu:t:"

// maxCallbackData is Telegram's hard limit for callback_data, in bytes.
const maxCallbackData = 64

func NewTokenStore(limit int) *TokenStore {
	if limit <= 0 {
		limit = 4096
	}
	return &TokenStore{
		byKey:  make(map[string]string),
		byData: make(map[string]string),
		limit:  limit,
	}
}

// Shorten returns callback data guaranteed to fit Telegram's limit. Data that
// already fits is returned unchanged, so short static menu entries stay
// human-readable in logs.
func (s *TokenStore) Shorten(data string) string {
	if len(data) <= maxCallbackData {
		return data
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse the key for identical data so repeated renders of the same list do
	// not grow the table.
	if key, ok := s.byData[data]; ok {
		return tokenPrefix + key
	}

	key := s.mintKey(data)
	s.byKey[key] = data
	s.byData[data] = key
	s.order = append(s.order, key)
	s.evictLocked()
	return tokenPrefix + key
}

// Resolve expands token callback data back to the original. Non-token data is
// returned unchanged. The second result is false only for an unknown token,
// which happens when a user taps a button from before a restart.
func (s *TokenStore) Resolve(data string) (string, bool) {
	if len(data) <= len(tokenPrefix) || data[:len(tokenPrefix)] != tokenPrefix {
		return data, true
	}
	key := data[len(tokenPrefix):]

	s.mu.RLock()
	defer s.mu.RUnlock()
	full, ok := s.byKey[key]
	if !ok {
		return "", false
	}
	return full, true
}

// mintKey derives a short key from the data itself, extending it on collision.
// Caller must hold the write lock.
func (s *TokenStore) mintKey(data string) string {
	sum := sha256.Sum256([]byte(data))
	enc := base64.RawURLEncoding.EncodeToString(sum[:])
	for n := 8; n <= len(enc); n++ {
		candidate := enc[:n]
		if existing, taken := s.byKey[candidate]; !taken || existing == data {
			return candidate
		}
	}
	return enc
}

// evictLocked drops the oldest entries once the table exceeds its limit, so a
// long-running bot cannot grow this map without bound.
func (s *TokenStore) evictLocked() {
	for len(s.order) > s.limit {
		oldest := s.order[0]
		s.order = s.order[1:]
		if data, ok := s.byKey[oldest]; ok {
			delete(s.byData, data)
			delete(s.byKey, oldest)
		}
	}
}
