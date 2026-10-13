package ddns

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test网页回显不是IP或不是200就拒绝(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("<html>not-an-ip</html>"))
	}))
	defer srv.Close()

	if _, err := fetchIPFromWeb(srv.URL, 4); err == nil {
		t.Fatal("HTTP 500 的 HTML 不应被当成 IP")
	}
}

func newWebIPSource(t *testing.T, body string, status int) *webIPSource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &webIPSource{URL: srv.URL}
}

func TestReview唯一合法来源可以直接采用(t *testing.T) {
	ip, err := fetchIPFromWebList([]*webIPSource{
		newWebIPSource(t, "203.0.113.10", http.StatusOK),
	}, 4)
	if err != nil || ip != "203.0.113.10" {
		t.Fatalf("唯一合法来源应直接采用，实际 %q %v", ip, err)
	}
}

func Test跳过无效来源后采用第一个合法IP(t *testing.T) {
	ip, err := fetchIPFromWebList([]*webIPSource{
		newWebIPSource(t, "<html>not-an-ip</html>", http.StatusInternalServerError),
		newWebIPSource(t, "203.0.113.10", http.StatusOK),
	}, 4)
	if err != nil || ip != "203.0.113.10" {
		t.Fatalf("应跳过无效来源并采用第一个合法 IP，实际 %q %v", ip, err)
	}
}
