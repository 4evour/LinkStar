package stun

import (
	"net"
	"strings"
	"testing"
	"time"

	"linkstar/modules/stun/model"
)

func Test内置DNS不可达时改用系统解析器(t *testing.T) {
	oldTimeout := dnsLookupTimeout
	oldServers := append([]string(nil), DefaultDNSServers...)
	oldCfg := networkConfig.Load()
	dnsLookupTimeout = 200 * time.Millisecond
	DefaultDNSServers = []string{"192.0.2.1"}
	t.Cleanup(func() {
		dnsLookupTimeout = oldTimeout
		DefaultDNSServers = oldServers
		networkConfig.Store(oldCfg)
	})
	networkConfig.Store(&model.NetworkConfig{})

	o, err := DetectOutboundIface()
	if err != nil {
		t.Fatal(err)
	}
	got, err := o.resolve("example.com:3478")
	if err != nil {
		t.Fatalf("内置 DNS 失败后应改用系统解析器: %v", err)
	}
	host, port, err := net.SplitHostPort(got)
	if err != nil || port != "3478" || net.ParseIP(host) == nil {
		t.Fatalf("解析结果应是 IPv4:3478，实际 %q", got)
	}
}

func Test自定义DNS失败时不退回系统解析器(t *testing.T) {
	oldTimeout := dnsLookupTimeout
	oldCfg := networkConfig.Load()
	dnsLookupTimeout = 200 * time.Millisecond
	t.Cleanup(func() {
		dnsLookupTimeout = oldTimeout
		networkConfig.Store(oldCfg)
	})
	networkConfig.Store(&model.NetworkConfig{DNSMode: model.ModeCustom, DNS: []string{"192.0.2.1"}})

	o, err := DetectOutboundIface()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.resolve("example.com:3478"); err == nil {
		t.Fatal("用户指定的 DNS 失败时不应悄悄改用系统解析器")
	}
}

func TestDialSTUN失败时带上服务器名(t *testing.T) {
	old := stunDialTimeout
	oldCfg := networkConfig.Load()
	stunDialTimeout = 200 * time.Millisecond
	t.Cleanup(func() {
		stunDialTimeout = old
		networkConfig.Store(oldCfg)
	})
	networkConfig.Store(&model.NetworkConfig{DNSMode: model.ModeSystem})

	// UDP 的 Dial 本身不代表对端在听，所以要走完一次 Binding，确认失败信息还带着服务器名。
	_, err := getSTUNServerDelay("127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("错误应包含服务器地址，实际 %v", err)
	}
}
