package passwordreset

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

// CodeStore stores generated codes and their associated email addresses.
type CodeStore struct {
	mu    sync.Mutex
	codes map[string]codeEntry
}

type codeEntry struct {
	email  string
	expiry time.Time
}

// NewCodeStore creates a new CodeStore.
func NewCodeStore() *CodeStore {
	return &CodeStore{
		codes: make(map[string]codeEntry),
	}
}

// GenerateCode generates a unique code and stores it with the email address.
func (cs *CodeStore) GenerateCode(email string, expiry time.Duration) (string, error) {
	b := make([]byte, 32) // Generate a 256-bit random code
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	code := base64.URLEncoding.EncodeToString(b)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.codes[code] = codeEntry{email: email, expiry: time.Now().Add(expiry)}

	return code, nil
}

// ValidateCode validates a code against the stored email address.
func (cs *CodeStore) ValidateCode(code, email string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entry, ok := cs.codes[code]
	if !ok {
		return false // Code not found
	}

	if entry.email != email {
		return false // Email mismatch
	}

	if time.Now().After(entry.expiry) {
		delete(cs.codes, code) //remove expired code
		return false           // code expired
	}

	delete(cs.codes, code) // Remove the code after successful validation
	return true
}

// CleanExpiredCodes removes expired codes from the store. This should be run periodically.
func (cs *CodeStore) CleanExpiredCodes() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now()
	for code, entry := range cs.codes {
		if now.After(entry.expiry) {
			delete(cs.codes, code)
		}
	}
}

// GetEmailFromCode retrieves the email associated with a code, intended for internal use.
func (cs *CodeStore) GetEmailFromCode(code string) (string, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entry, ok := cs.codes[code]
	if !ok {
		return "", errors.New("code not found")
	}
	return entry.email, nil
}
