package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordHash(t *testing.T) {
	hash := hashPassword("correct horse")
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") || strings.Contains(hash, "correct horse") {
		t.Errorf("hash = %q", hash)
	}
	if hashPassword("correct horse") == hash {
		t.Error("two hashes of one password must differ by their salt")
	}
	tests := []struct {
		name     string
		hash     string
		password string
		want     bool
	}{
		{"right password", hash, "correct horse", true},
		{"wrong password", hash, "correct horsf", false},
		{"empty password", hash, "", false},
		{"not a hash", "plain text", "correct horse", false},
		{"other algorithm", strings.Replace(hash, "argon2id", "argon2i", 1), "correct horse", false},
		{"zero passes", strings.Replace(hash, "t=2", "t=0", 1), "correct horse", false},
		{"broken salt", strings.Replace(hash, "$m=19456,t=2,p=1$", "$m=19456,t=2,p=1$!!", 1), "correct horse", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkPassword(tt.hash, tt.password); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanEmail(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"  Student@Example.COM ", "student@example.com", true},
		{"a.b+c@school.ac.in", "a.b+c@school.ac.in", true},
		{"", "", false},
		{"student", "", false},
		{"@example.com", "", false},
		{"a@localhost", "", false},
		{"Student <a@b.com>", "", false},
		{"a@b.com, c@d.com", "", false},
		{strings.Repeat("a", 250) + "@b.com", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := cleanEmail(tt.in)
			if (err == nil) != tt.ok || got != tt.want {
				t.Errorf("got %q, %v", got, err)
			}
		})
	}
}

func TestHashTokenIsNotTheToken(t *testing.T) {
	if got := hashToken("abc"); len(got) != 64 || got == "abc" || got != hashToken("abc") {
		t.Errorf("hash = %q", got)
	}
}

func TestThrottle(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	throttle := newThrottle()
	throttle.now = func() time.Time { return now }

	for range triesPerEmail {
		if !throttle.allowed("a@b.com", "1.1.1.1") {
			t.Fatal("blocked too early")
		}
		throttle.failed("a@b.com", "1.1.1.1")
	}
	if throttle.allowed("a@b.com", "2.2.2.2") {
		t.Error("the email should be blocked from any address")
	}
	if !throttle.allowed("c@d.com", "1.1.1.1") {
		t.Error("the address isn't at its limit yet")
	}

	// Many emails from one address.
	for i := range triesPerIP {
		throttle.failed(strings.Repeat("x", i+1)+"@b.com", "3.3.3.3")
	}
	if throttle.allowed("new@b.com", "3.3.3.3") {
		t.Error("the address should be blocked for any email")
	}

	now = now.Add(throttleWindow)
	if !throttle.allowed("a@b.com", "3.3.3.3") {
		t.Error("the window has passed")
	}
	throttle.failed("a@b.com", "1.1.1.1")
	if throttle.tries["email:a@b.com"].failures != 1 {
		t.Errorf("a new window starts from one: %+v", throttle.tries["email:a@b.com"])
	}
}

func TestThrottleSweepsOldEntries(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	throttle := newThrottle()
	throttle.now = func() time.Time { return now }
	for i := range throttleSweepAt {
		throttle.tries[strings.Repeat("k", i+1)] = window{start: now, failures: 1}
	}
	now = now.Add(throttleWindow)
	throttle.failed("a@b.com", "1.1.1.1")
	if len(throttle.tries) != 2 {
		t.Errorf("entries = %d, want only the two new ones", len(throttle.tries))
	}
}
