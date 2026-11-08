package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// argon2id with OWASP's minimum settings: 19 MiB, 2 passes, 1 thread. Kept in
// each hash, so they can be raised later without breaking old passwords.
const (
	argonMemoryKiB = 19 * 1024
	argonPasses    = 2
	argonThreads   = 1
	argonKeyBytes  = 32
	saltBytes      = 16
)

// Each hash holds 19 MiB while it runs, so only a few run at once: a burst of
// logins waits instead of using up the phone's memory.
var hashing = make(chan struct{}, 4)

func derive(password string, salt []byte, passes, memoryKiB uint32, threads uint8, keyBytes uint32) []byte {
	hashing <- struct{}{}
	defer func() { <-hashing }()
	return argon2.IDKey([]byte(password), salt, passes, memoryKiB, threads, keyBytes)
}

// hashPassword returns the password's hash in the usual
// $argon2id$v=19$m=...,t=...,p=...$salt$key form.
func hashPassword(password string) string {
	salt := make([]byte, saltBytes)
	_, _ = rand.Read(salt) // never fails
	key := derive(password, salt, argonPasses, argonMemoryKiB, argonThreads, argonKeyBytes)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonPasses, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// checkPassword says whether password matches a hash from hashPassword. The
// final comparison takes the same time however many bytes match.
func checkPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false
	}
	var memoryKiB, passes uint32
	var threads uint8
	// argon2 panics on zero passes or threads.
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memoryKiB, &passes, &threads); err != nil || passes == 0 || threads == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return false
	}
	got := derive(password, salt, passes, memoryKiB, threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(got, key) == 1
}

// noAccountHash is checked when an email has no account, so a login takes the
// same time either way and doesn't reveal which emails have accounts.
var noAccountHash = sync.OnceValue(func() string { return hashPassword("no account has this password") })
