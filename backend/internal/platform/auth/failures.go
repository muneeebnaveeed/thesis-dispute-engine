package auth

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/errs"
	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/httpserver"
)

// ErrTooManyFailures answers a peer that keeps presenting credentials that do not resolve.
var ErrTooManyFailures = errs.New(errs.RateLimited, "rate-limited", "too many failed authentication attempts from this address")

// FailureLimiter slows credential guessing: a peer that presented an invalid credential more than perMinute
// times in the last minute is refused before any lookup. Per process on purpose; it does not need to be exact,
// only to make brute force impractical, and it must not add a database write per anonymous request.
type FailureLimiter struct {
	mu        sync.Mutex
	perMinute int
	hits      map[string][]time.Time
	lastSweep time.Time
}

// NewFailureLimiter returns a limiter; perMinute <= 0 disables it.
func NewFailureLimiter(perMinute int) *FailureLimiter {
	return &FailureLimiter{perMinute: perMinute, hits: map[string][]time.Time{}, lastSweep: time.Now()}
}

// Middleware must sit after Bearer: it reads the outcome Bearer recorded and counts the failures per peer.
func (f *FailureLimiter) Middleware(next http.Handler) http.Handler {
	if f.perMinute <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only bearer credentials can be guessed here; anonymous and service-key traffic is neither counted nor blocked,
		// so a guessing burst from a shared address (or the frontend server's own host) cannot take the app down.
		if r.Header.Get("Authorization") == "" {
			next.ServeHTTP(w, r)
			return
		}
		peer := peerOf(r.RemoteAddr)
		if f.blocked(peer) {
			httpserver.Annotate(r.Context(), "auth", "blocked")
			httpserver.WriteProblem(w, r, httpserver.ProblemFrom(r.Context(), r.URL.Path, ErrTooManyFailures).WithRetryAfter(60), nil)
			return
		}
		if _, presented := r.Context().Value(ctxKey{}).(error); presented {
			f.record(peer)
		}
		next.ServeHTTP(w, r)
	})
}

func (f *FailureLimiter) blocked(peer string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sweepLocked()
	return len(f.recentLocked(peer)) >= f.perMinute
}

func (f *FailureLimiter) record(peer string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits[peer] = append(f.recentLocked(peer), time.Now())
}

func (f *FailureLimiter) recentLocked(peer string) []time.Time {
	cutoff := time.Now().Add(-time.Minute)
	all := f.hits[peer]
	i := 0
	for i < len(all) && all[i].Before(cutoff) {
		i++
	}
	return all[i:]
}

// sweepLocked drops idle peers so the map cannot grow without bound.
func (f *FailureLimiter) sweepLocked() {
	if time.Since(f.lastSweep) < time.Minute {
		return
	}
	f.lastSweep = time.Now()
	for peer := range f.hits {
		if len(f.recentLocked(peer)) == 0 {
			delete(f.hits, peer)
		}
	}
}

func peerOf(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}
