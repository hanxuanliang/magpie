package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A renewal that leaves the sign-in due isn't made again by the next
// listing, request or keep-alive, as a failed one isn't. A plugin with an
// hour's refreshLead that, unable to renew, gave back only an expires half
// an hour out was renewed again by everything that used the account, and
// each save told magpie the sign-ins changed, which listed every provider
// again: every plugin's vendor was asked every few seconds while magpie
// ran. One that brought no new token isn't saved; one that brought a new
// token is, and isn't renewed again at once; one that gave back nothing
// isn't asked again at once either.
func TestRenewalLeftDueIsNotMadeAgain(t *testing.T) {
	sandbox(t)
	logf := filepath.Join(t.TempDir(), "renewals")
	t.Setenv("RENEW_LOG", logf)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	abs, _ := filepath.Abs("testdata/renew-due/index.js")
	if _, err := Add(ctx, abs); err != nil {
		t.Fatal(err)
	}
	soon := time.Now().Add(time.Minute).UnixMilli() // inside the hour's lead
	account := func(who, refresh string) string {
		t.Helper()
		k, err := Import(ctx, "dueco", map[string]any{"type": "oauth", "refresh": refresh, "access": "old-" + who, "expires": soon, "accountId": who})
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	keep := account("keep@due", "d-keep")
	fresh := account("new@due", "d-new")
	none := account("none@due", "d-none")
	stored := func(key string) map[string]any {
		var all map[string]map[string]any
		b, _ := os.ReadFile(AuthPath())
		json.Unmarshal(b, &all)
		return all[key]
	}
	renewals := func(prefix string) int {
		b, _ := os.ReadFile(logf)
		n := 0
		for _, l := range strings.Fields(string(b)) {
			if strings.HasPrefix(l, prefix) {
				n++
			}
		}
		return n
	}

	// a listing renews each due sign-in before its models hook runs
	if _, err := Providers(ctx); err != nil {
		t.Fatal(err)
	}
	first := time.Now()
	for _, p := range []string{"d-keep", "d-new", "d-none"} {
		if n := renewals(p); n != 1 {
			t.Fatalf("%s renewed %d times by one listing, want 1", p, n)
		}
	}
	// and the next listing, a moment later, renews none of them again
	if _, err := Providers(ctx); err != nil {
		t.Fatal(err)
	}
	k, err := KeepSignedIn(ctx, "dueco", keep)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(first) < 25*time.Second { // RENEW_RETRY is 30s
		for _, p := range []string{"d-keep", "d-new", "d-none"} {
			if n := renewals(p); n != 1 {
				t.Errorf("%s renewed %d times: a renewal that left it due was made again at once", p, n)
			}
		}
		// the keep-alive is told why, and backs off as from a failure
		if !k.Due || k.Renewed || !k.Waiting || !strings.Contains(k.Err, "refreshLead") {
			t.Errorf("keep-alive on the still-due sign-in: %+v", k)
		}
	} else {
		t.Logf("the listings were %s apart, past RENEW_RETRY: renewed again is right", time.Since(first))
	}

	// nothing new: not saved, the sign-in as it was
	if a := stored(keep); a["access"] != "old-keep@due" || a["refresh"] != "d-keep" || int64(a["expires"].(float64)) != soon {
		t.Errorf("a renewal with no new token was saved: %v", a)
	}
	// a new token: kept, though due
	if a := stored(fresh); a["access"] != "old-new@due+" || a["refresh"] != "d-new+" {
		t.Errorf("a renewal's new token wasn't kept: %v", a)
	}
	if a := stored(none); a["access"] != "old-none@due" || int64(a["expires"].(float64)) != soon {
		t.Errorf("a renewal that gave back nothing changed the sign-in: %v", a)
	}
}
