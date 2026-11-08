package auth

import (
	"sync"
	"time"
)

// Wrong passwords allowed in each window, by email and by IP address. An IP
// gets more, because a hostel or a phone network puts many students behind one.
const (
	throttleWindow = 15 * time.Minute
	triesPerEmail  = 5
	triesPerIP     = 20
	// Past this many entries, old ones are swept out.
	throttleSweepAt = 10_000
)

// throttle counts wrong passwords, so guessing one is slow.
// shortcut: counts live in this process's memory, so a restart forgets them and two API processes wouldn't share them. Move them to Postgres if the API ever runs as more than one process.
type throttle struct {
	mu    sync.Mutex
	tries map[string]window
	now   func() time.Time
}

type window struct {
	start    time.Time
	failures int
}

func newThrottle() *throttle {
	return &throttle{tries: map[string]window{}, now: time.Now}
}

// allowed says whether this email and IP may try a password now.
func (t *throttle) allowed(email, ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.failures("email:"+email) < triesPerEmail && t.failures("ip:"+ip) < triesPerIP
}

// failed counts one wrong password against both.
func (t *throttle) failed(email, ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.tries) >= throttleSweepAt {
		for key, w := range t.tries {
			if t.now().Sub(w.start) >= throttleWindow {
				delete(t.tries, key)
			}
		}
	}
	for _, key := range []string{"email:" + email, "ip:" + ip} {
		w := window{start: t.now(), failures: 1}
		if t.failures(key) > 0 {
			w = t.tries[key]
			w.failures++
		}
		t.tries[key] = w
	}
}

// failures counts a key's wrong passwords in its current window. The caller holds mu.
func (t *throttle) failures(key string) int {
	w, ok := t.tries[key]
	if !ok || t.now().Sub(w.start) >= throttleWindow {
		return 0
	}
	return w.failures
}
