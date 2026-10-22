// Package auth — JWT signing/verification (HS256) + password hashing (argon2id).
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// ── Password (argon2id) ──────────────────────────────────────────────

// Argon2 parameters (rekomendasi OWASP: m=64MiB, t=3, p=1).
const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 1
	argonKeyLength   = 32
	argonSaltLength  = 16
)

// HashPassword — hash password dengan argon2id, format:
// $argon2id$v=19$m=65536,t=3,p=1$<salt-b64>$<hash-b64>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2: salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, uint8(argonParallelism), argonKeyLength)
	enc := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonIterations, argonParallelism,
		enc(salt), enc(hash)), nil
}

// DummyHash — pre-computed hash untuk verifikasi login konstan waktu saat user tak ditemukan (SEC-05).
var DummyHash, _ = HashPassword("skyward-dummy-auth-constant-time-salt-padding")

// VerifyPassword — verifikasi password terhadap string hash argon2id.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// format: ["", "argon2id", "v=19", "m=...,t=...,p=...", "salt", "hash"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("argon2: unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("argon2: parse version: %w", err)
	}
	var memory, iterations, parallelism uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, fmt.Errorf("argon2: parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("argon2: decode salt: %w", err)
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("argon2: decode hash: %w", err)
	}
	computed := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(computed, expected) == 1, nil
}

// ── JWT (HS256) ──────────────────────────────────────────────────────

// Claims — klaim JWT minimal untuk Skyward.
type Claims struct {
	Sub          string `json:"sub"`                      // public.users.id
	Username     string `json:"username"`                 // display / name
	Exp          int64  `json:"exp"`                      // expiry unix timestamp
	Iat          int64  `json:"iat"`                      // issued at
	TokenVersion int    `json:"token_version,omitempty"` // session token version
}

var (
	revocationMu      sync.RWMutex
	userTokenVersions = make(map[string]int)
	revokedBefore     = make(map[string]int64)
)

// CurrentTokenVersion — versi token aktif untuk user_id (default: 1).
func CurrentTokenVersion(userID string) int {
	revocationMu.RLock()
	defer revocationMu.RUnlock()
	v, ok := userTokenVersions[userID]
	if !ok {
		return 1
	}
	return v
}

// RevokeUserTokens — batalkan seluruh token aktif milik userID (mis. saat reset password atau hapus akun).
// Menaikkan token_version dan mencatat cutoff timestamp (SEC-02).
func RevokeUserTokens(userID string) {
	if userID == "" {
		return
	}
	revocationMu.Lock()
	defer revocationMu.Unlock()
	current := userTokenVersions[userID]
	if current == 0 {
		current = 1
	}
	userTokenVersions[userID] = current + 1
	now := time.Now().Unix()
	revokedBefore[userID] = now

	// Prune entri lama jika map berkembang melebihi ambang
	if len(revokedBefore) > 1000 {
		cutoff := now - 24*3600 // token berlaku maks 24 jam
		for id, t := range revokedBefore {
			if t < cutoff {
				delete(revokedBefore, id)
			}
		}
	}
}

// IsRevoked — periksa apakah token/klaim sudah dibatalkan (revoked).
func IsRevoked(claims *Claims) bool {
	if claims == nil || claims.Sub == "" {
		return true
	}
	revocationMu.RLock()
	defer revocationMu.RUnlock()

	// 1. Cek token_version jika diset pada klaim
	if claims.TokenVersion > 0 {
		if expectedVer, exists := userTokenVersions[claims.Sub]; exists {
			return claims.TokenVersion < expectedVer
		}
		return false
	}

	// 2. Fallback untuk token tanpa token_version: cek cutoff timestamp
	if cutoff, exists := revokedBefore[claims.Sub]; exists {
		return claims.Iat <= cutoff
	}

	return false
}

// ResetRevocationsForTest — reset state revokasi (khusus untuk testing).
func ResetRevocationsForTest() {
	revocationMu.Lock()
	defer revocationMu.Unlock()
	userTokenVersions = make(map[string]int)
	revokedBefore = make(map[string]int64)
}

// Sign — buat JWT HS256 dari claims + secret.
func Sign(claims Claims, secret []byte) (string, error) {
	if len(secret) < 32 {
		return "", fmt.Errorf("jwt: secret too short (min 32 bytes)")
	}
	header := []byte(`{"alg":"HS256","typ":"JWT"}`)
	claims.Iat = time.Now().Unix()
	if claims.Exp == 0 {
		claims.Exp = time.Now().Add(24 * time.Hour).Unix()
	}
	if claims.TokenVersion == 0 && claims.Sub != "" {
		claims.TokenVersion = CurrentTokenVersion(claims.Sub)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("jwt: marshal claims: %w", err)
	}
	enc := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	sigInput := enc(header) + "." + enc(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	return sigInput + "." + enc(mac.Sum(nil)), nil
}

// Parse — verifikasi JWT HS256, parse claims.
func Parse(token string, secret []byte) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("jwt: malformed")
	}
	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode sig: %w", err)
	}
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, fmt.Errorf("jwt: invalid signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("jwt: decode payload: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("jwt: parse claims: %w", err)
	}
	if claims.Exp <= 0 || time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("jwt: expired")
	}
	return &claims, nil
}
