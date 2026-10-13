package stun

import (
	"sync"
	"testing"
)

func Test公网IP的读写不会互相踩(t *testing.T) {
	old := PublicIP()
	t.Cleanup(func() {
		networkMu.Lock()
		Runtime.Network.PublicIP = old
		networkMu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 200; n++ {
				setObservedAddress("192.0.2.10", "203.0.113.10")
				if ip := PublicIP(); ip != "203.0.113.10" && ip != old && ip != "" {
					t.Errorf("读到意外的公网 IP %q", ip)
				}
				_ = NetworkSnapshot()
			}
		}()
	}
	wg.Wait()
	if PublicIP() != "203.0.113.10" {
		t.Fatalf("并发写完后的公网 IP = %q", PublicIP())
	}
}
