package cli

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// RFC 7636 appendix B test vector.
func TestPkceChallengeMatchesRFC7636Vector(t *testing.T) {
	got := pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("got %s", got)
	}
}

const testAgent = "claude-code"

func TestBuildAuthorizeURLCarriesPKCEAndState(t *testing.T) {
	u, err := url.Parse(buildAuthorizeURL("https://x/authorize", "http://127.0.0.1:1234/callback", "chal", "st", testAgent))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != "mobilecli" || q.Get("code_challenge") != "chal" || q.Get("code_challenge_method") != "S256" ||
		q.Get("state") != "st" || q.Get("redirect_uri") != "http://127.0.0.1:1234/callback" || q.Get("response_type") != "code" ||
		q.Get("agent") != testAgent {
		t.Fatalf("bad query: %s", u.RawQuery)
	}
}

func TestWaitForCallbackReturnsCodeAndState(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		time.Sleep(50 * time.Millisecond)
		callbackURL := "http://" + listener.Addr().String() + "/callback?code=abc&state=xyz"
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, callbackURL, nil)
		if reqErr != nil {
			return
		}
		resp, getErr := http.DefaultClient.Do(req)
		if getErr != nil {
			return
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("failed to close callback response: %v", closeErr)
		}
	}()

	cb, err := waitForCallback(ctx, listener)
	if err != nil {
		t.Fatal(err)
	}
	if cb.code != "abc" || cb.state != "xyz" {
		t.Fatalf("got %+v", cb)
	}
}

func TestExchangeCodeSendsVerifierAndReturnsToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if parseErr := r.ParseForm(); parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("code_verifier") != "ver" || r.PostForm.Get("code") != "abc" || r.PostForm.Get("client_id") != "mobilecli" {
			w.WriteHeader(http.StatusBadRequest)
			writeTestResponse(t, w, `{"error":"invalid_grant"}`)
			return
		}
		writeTestResponse(t, w, `{"access_token":"mob_token","token_type":"bearer"}`)
	}))
	defer server.Close()

	token, err := exchangeCode(server.URL, "abc", "ver", "http://127.0.0.1:1/callback")
	if err != nil {
		t.Fatal(err)
	}
	if token != "mob_token" {
		t.Fatalf("got %s", token)
	}
}

func TestDetectAgentRecognisesClaudeCode(t *testing.T) {
	if got := detectAgent(envWith(map[string]string{"CLAUDECODE": "1"})); got != "claude-code" {
		t.Fatalf("got %q", got)
	}
	if got := detectAgent(envWith(nil)); got != "" {
		t.Fatalf("expected empty for a plain terminal, got %q", got)
	}
}

func writeTestResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("failed to write test response: %v", err)
	}
}
