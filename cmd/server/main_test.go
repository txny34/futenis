package main

import "testing"

func TestApplyEnv(t *testing.T) {
	addr, origins, trust := "127.0.0.1:8091", "localhost:8090", false
	applyEnv(&addr, &origins, &trust)
	if addr != "127.0.0.1:8091" || origins != "localhost:8090" || trust {
		t.Fatalf("no env must keep flags: %q %q %v", addr, origins, trust)
	}
	t.Setenv("PORT", "10000")
	t.Setenv("FUTENIS_ORIGINS", "futenis.vercel.app")
	t.Setenv("FUTENIS_TRUST_PROXY", "1")
	applyEnv(&addr, &origins, &trust)
	if addr != ":10000" || origins != "futenis.vercel.app" || !trust {
		t.Fatalf("env not applied: %q %q %v", addr, origins, trust)
	}
}
