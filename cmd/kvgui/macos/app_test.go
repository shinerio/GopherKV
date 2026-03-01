package main

import (
	"context"
	"testing"
)

func newTestApp() *App {
	app := NewApp()
	app.startup(context.Background())
	return app
}

func TestNewApp_NotNil(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("expected non-nil App")
	}
}

func TestDisconnect_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.Disconnect()
	if !res.Success {
		t.Fatalf("Disconnect on nil client should succeed, got: %s", res.Message)
	}
}

func TestSetKey_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.SetKey("k", "v", 0)
	if res.Success {
		t.Fatal("SetKey without connection should fail")
	}
	if res.Message != "not connected" {
		t.Fatalf("unexpected message: %s", res.Message)
	}
}

func TestGetKey_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.GetKey("k")
	if res.Success {
		t.Fatal("GetKey without connection should fail")
	}
}

func TestDeleteKey_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.DeleteKey("k")
	if res.Success {
		t.Fatal("DeleteKey without connection should fail")
	}
}

func TestExistsKey_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.ExistsKey("k")
	if res.Success {
		t.Fatal("ExistsKey without connection should fail")
	}
}

func TestGetTTL_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.GetTTL("k")
	if res.Success {
		t.Fatal("GetTTL without connection should fail")
	}
}

func TestGetStats_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.GetStats()
	if res.Success {
		t.Fatal("GetStats without connection should fail")
	}
}

func TestTriggerSnapshot_WhenNotConnected(t *testing.T) {
	app := newTestApp()
	res := app.TriggerSnapshot()
	if res.Success {
		t.Fatal("TriggerSnapshot without connection should fail")
	}
}

func TestConnect_InvalidAddress(t *testing.T) {
	app := newTestApp()
	res := app.Connect("127.0.0.1", 1)
	if res.Success {
		t.Fatal("Connect to invalid address should fail")
	}
}
