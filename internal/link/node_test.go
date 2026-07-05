package link

import "testing"

func TestParseVLESSReality(t *testing.T) {
	node, err := Parse("vless://00000000-0000-0000-0000-000000000000@example.com:443?security=reality&sni=example.com&fp=chrome&pbk=public&sid=short&type=tcp&flow=xtls-rprx-vision&encryption=none#main")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node.Name != "main" {
		t.Fatalf("Name = %q", node.Name)
	}
	if node.Protocol != "vless" {
		t.Fatalf("Protocol = %q", node.Protocol)
	}
	if node.Server != "example.com" || node.Port != 443 {
		t.Fatalf("server = %s:%d", node.Server, node.Port)
	}
	if node.Security != "reality" || node.SNI != "example.com" || node.PublicKey != "public" || node.ShortID != "short" {
		t.Fatalf("unexpected reality fields: %#v", node)
	}
	if node.Flow != "xtls-rprx-vision" || node.Encryption != "none" {
		t.Fatalf("unexpected user fields: flow=%q encryption=%q", node.Flow, node.Encryption)
	}
}

func TestParseVLESSRequiresRealityPublicKey(t *testing.T) {
	_, err := Parse("vless://00000000-0000-0000-0000-000000000000@example.com:443?security=reality&type=tcp#main")
	if err == nil {
		t.Fatal("expected error")
	}
}
