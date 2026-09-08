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

func TestBuildAuthorizeURLCarriesPKCEAndState(t *testing.T) {
	u, err := url.Parse(buildAuthorizeURL("https://x/authorize", "http://127.0.0.1:1234/callback", "chal", "st"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != "mobilecli" || q.Get("code_challenge") != "chal" || q.Get("code_challenge_method") != "S256" ||
		q.Get("state") != "st" || q.Get("redirect_uri") != "http://127.0.0.1:1234/callback" || q.Get("response_type") != "code" {
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
		resp, err := http.Get("http://" + listener.Addr().String() + "/callback?code=abc&state=xyz")
		if err == nil {
			resp.Body.Close()
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
		_ = r.ParseForm()
		if r.PostForm.Get("code_verifier") != "ver" || r.PostForm.Get("code") != "abc" || r.PostForm.Get("client_id") != "mobilecli" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Write([]byte(`{"access_token":"mob_token","token_type":"bearer"}`))
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
