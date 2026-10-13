package ddns

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func Test内置回显至少两个源一致才采用(t *testing.T) {
	newSrc := func(body string, status int) *webIPSource {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(srv.Close)
		return &webIPSource{URL: srv.URL}
	}

	if ip, err := fetchIPFromWebList([]*webIPSource{
		newSrc("203.0.113.10", http.StatusOK),
		newSrc("203.0.113.11", http.StatusOK),
		newSrc("203.0.113.10", http.StatusOK),
	}, 4); err != nil || ip != "203.0.113.10" {
		t.Fatalf("两个源都给出 203.0.113.10 时应采用它，实际 %q %v", ip, err)
	}

	if _, err := fetchIPFromWebList([]*webIPSource{
		newSrc("203.0.113.10", http.StatusOK),
		newSrc("203.0.113.11", http.StatusOK),
	}, 4); err == nil || !strings.Contains(err.Error(), "没有两个一致") {
		t.Fatalf("结果互相矛盾时应拒绝，实际 %v", err)
	}
}
