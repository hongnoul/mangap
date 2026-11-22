package anilist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hongnoul/mangap/constant"
)

// CallbackPort is the fixed localhost port of the OAuth redirect.
// AniList allows a single redirect URL per client, so it cannot be random.
const CallbackPort = 47563

// RedirectURL must match the redirect URL registered on the mangap AniList client.
var RedirectURL = "http://localhost:" + strconv.Itoa(CallbackPort) + "/callback"

// ClientID returns the AniList client used for login: the user's own client
// (anilist.id) if set, else the client bundled with mangap.
func ClientID() string {
	if id := (&Anilist{}).id(); id != "" {
		return id
	}
	return constant.AnilistClientID
}

// ImplicitAuthURL is the implicit-grant authorize URL. No client secret is
// needed, so every mangap user can share one public client.
func ImplicitAuthURL(clientID string) string {
	return "https://anilist.co/api/v2/oauth/authorize?client_id=" + url.QueryEscape(clientID) + "&response_type=token"
}

// Token is an AniList access token with its expiry.
type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

// ParseCallback extracts a token from a callback URL or its fragment
// ("access_token=...&expires_in=..."). Users without a browser on this machine
// can paste the address bar URL instead.
func ParseCallback(raw string) (Token, error) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "#"); i >= 0 {
		raw = raw[i+1:]
	} else if i := strings.Index(raw, "?"); i >= 0 {
		raw = raw[i+1:]
	}

	values, err := url.ParseQuery(raw)
	if err != nil {
		return Token{}, err
	}
	if e := values.Get("error"); e != "" {
		return Token{}, fmt.Errorf("anilist denied access: %s %s", e, values.Get("error_description"))
	}

	access := values.Get("access_token")
	if access == "" {
		// A bare token pasted on its own.
		if !strings.ContainsAny(raw, "=&") && len(raw) > 20 {
			return Token{AccessToken: raw}, nil
		}
		return Token{}, errors.New("no access_token found")
	}

	t := Token{AccessToken: access}
	if s, err := strconv.Atoi(values.Get("expires_in")); err == nil && s > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(s) * time.Second)
	}
	return t, nil
}

// The token lives in the URL fragment, which browsers never send to servers,
// so this page posts it back to /token.
const callbackPage = `<!doctype html><meta charset="utf-8"><title>mangap</title>
<body style="font-family:system-ui;text-align:center;margin-top:20vh">
<h2 id="m">Connecting mangap to AniList…</h2>
<script>
fetch("/token", {method: "POST", body: location.hash.slice(1) || location.search.slice(1)})
  .then(r => r.text())
  .then(t => document.getElementById("m").textContent = t)
  .catch(e => document.getElementById("m").textContent = "Failed: " + e);
</script>`

// ListenForToken serves the redirect on localhost and returns the token once
// the browser delivers it. It fails fast if the port is unavailable.
func ListenForToken(ctx context.Context) (Token, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(CallbackPort))
	if err != nil {
		return Token{}, fmt.Errorf("cannot listen on port %d: %w", CallbackPort, err)
	}

	result := make(chan Token, 1)
	failure := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(callbackPage))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := make([]byte, 8192)
		n, _ := r.Body.Read(body)
		token, err := ParseCallback(string(body[:n]))
		if err != nil {
			http.Error(w, "Login failed: "+err.Error(), http.StatusBadRequest)
			select {
			case failure <- err:
			default:
			}
			return
		}
		_, _ = w.Write([]byte("✓ mangap is connected to AniList. You can close this tab."))
		select {
		case result <- token:
		default:
		}
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	select {
	case t := <-result:
		return t, nil
	case err := <-failure:
		return Token{}, err
	case <-ctx.Done():
		return Token{}, ctx.Err()
	}
}

// Viewer returns the AniList username the token belongs to, verifying it works.
func Viewer(token string) (string, error) {
	body := strings.NewReader(`{"query":"{ Viewer { name } }"}`)
	req, err := http.NewRequest(http.MethodPost, "https://graphql.anilist.co", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token rejected by anilist (status %d)", resp.StatusCode)
	}

	var out struct {
		Data struct {
			Viewer struct {
				Name string `json:"name"`
			} `json:"Viewer"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Data.Viewer.Name, nil
}
