package app

import (
	"context"
	"testing"
)

func TestNewLoadsBothStores(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())

	a, err := New()
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if a.filterStore() == nil {
		t.Error("filterStore() is nil")
	}
	if a.profileStore() == nil {
		t.Error("profileStore() is nil")
	}
}

func TestNewOnAFreshMachineHasTheBuiltinFilters(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())

	a, err := New()
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if got := len(a.filterStore().All()); got != 18 {
		t.Errorf("a fresh install has %d filters, want the 18 built-ins", got)
	}
}

func TestLiveConnectionRegistry(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, err := New()
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	if _, ok := a.live("nobody"); ok {
		t.Error("live() found a connection that was never opened")
	}

	a.setLive("dc01", &liveConn{state: ConnectionState{ProfileID: "dc01", Connected: true}})
	got, ok := a.live("dc01")
	if !ok {
		t.Fatal("live() lost the connection just registered")
	}
	if !got.state.Connected {
		t.Error("the registered connection is not marked connected")
	}

	a.dropLive("dc01")
	if _, ok := a.live("dc01"); ok {
		t.Error("dropLive() left the connection behind")
	}
}

func TestDropLiveCancelsTheConnectionContext(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	var cancelled bool
	a.setLive("dc01", &liveConn{cancel: func() { cancelled = true }})
	a.dropLive("dc01")

	if !cancelled {
		t.Error("dropLive() did not cancel the connection's context, so its socket stays open")
	}
}

// Shutdown must close every connection, not just the current one.
func TestShutdownClosesEverything(t *testing.T) {
	t.Setenv("LDAPPER_CONFIG_DIR", t.TempDir())
	a, _ := New()

	var closed int
	for _, id := range []string{"dc01", "dc02"} {
		a.setLive(id, &liveConn{
			state:  ConnectionState{ProfileID: id},
			cancel: func() { closed++ },
		})
	}

	a.Shutdown(context.TODO())

	if closed != 2 {
		t.Errorf("%d connections were closed, want 2", closed)
	}
	if _, ok := a.live("dc01"); ok {
		t.Error("Shutdown() left a connection in the registry")
	}
}
