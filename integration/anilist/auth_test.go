package anilist

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseCallback(t *testing.T) {
	cases := map[string]string{
		"http://localhost:47563/callback#access_token=abc.def&token_type=Bearer&expires_in=31536000": "abc.def",
		"access_token=xyz&expires_in=10":                        "xyz",
		"  eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.payload.sig \n": "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.payload.sig",
	}
	for in, want := range cases {
		got, err := ParseCallback(in)
		if err != nil || got.AccessToken != want {
			t.Errorf("ParseCallback(%q) = %q, %v; want %q", in, got.AccessToken, err, want)
		}
	}

	tok, _ := ParseCallback("access_token=a&expires_in=3600")
	if d := time.Until(tok.ExpiresAt); d < 59*time.Minute || d > time.Hour {
		t.Errorf("expiry not parsed: %v", d)
	}

	for _, bad := range []string{"", "http://localhost/callback#error=access_denied", "foo=bar"} {
		if _, err := ParseCallback(bad); err == nil {
			t.Errorf("ParseCallback(%q) should fail", bad)
		}
	}
}

func TestListenForToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got := make(chan Token, 1)
	go func() {
		tok, err := ListenForToken(ctx)
		if err != nil {
			t.Error(err)
		}
		got <- tok
	}()

	base := "http://127.0.0.1:" + strconv.Itoa(CallbackPort)
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		if resp, err = http.Get(base + "/callback"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), "fetch(\"/token\"") {
		t.Fatal("callback page does not post the fragment back")
	}

	resp, err = http.Post(base+"/token", "text/plain", strings.NewReader("access_token=secret123&expires_in=60"))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("post token: %v %v", err, resp)
	}

	if tok := <-got; tok.AccessToken != "secret123" {
		t.Fatalf("got %q", tok.AccessToken)
	}
}
