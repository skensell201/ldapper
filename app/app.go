package app

import (
	"context"
	"fmt"
	"sync"

	"github.com/skensell201/ldapper/internal/config"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/profiles"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/session"
)

// liveConn is one open connection and what the interface knows about it.
type liveConn struct {
	conn   *session.Conn
	info   schema.Info
	state  ConnectionState
	cancel context.CancelFunc
}

// App is the object Wails binds. Every exported method on it becomes a
// function the interface can call.
type App struct {
	ctx      context.Context
	filters  *filters.Store
	profiles *profiles.Store

	mu    sync.RWMutex
	conns map[string]*liveConn
}

// New loads the stores. It is called before the window exists, so it must not
// depend on anything Wails provides.
func New() (*App, error) {
	filtersPath, err := config.FiltersPath()
	if err != nil {
		return nil, err
	}
	connectionsPath, err := config.ConnectionsPath()
	if err != nil {
		return nil, err
	}

	f, err := filters.NewStore(filtersPath)
	if err != nil {
		return nil, fmt.Errorf("app: cannot load the filter library: %w", err)
	}
	p, err := profiles.NewStore(connectionsPath)
	if err != nil {
		return nil, fmt.Errorf("app: cannot load the saved connections: %w", err)
	}

	return &App{filters: f, profiles: p, conns: map[string]*liveConn{}}, nil
}

// Startup receives the Wails context, which is what event emission needs.
func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

// Shutdown closes every open connection. Leaving one behind would hold a
// socket open past the window closing.
func (a *App) Shutdown(context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for id, c := range a.conns {
		if c.cancel != nil {
			c.cancel()
		}
		delete(a.conns, id)
	}
}

// filterStore and profileStore are accessors for the rest of this
// package. They are deliberately unexported: every exported method on App
// becomes a function the interface can call, and binding a mutex-bearing
// store would drag its internals into the generated TypeScript.
func (a *App) filterStore() *filters.Store { return a.filters }

func (a *App) profileStore() *profiles.Store { return a.profiles }

func (a *App) live(id string) (*liveConn, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	c, ok := a.conns[id]
	return c, ok
}

func (a *App) setLive(id string, c *liveConn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.conns[id] = c
}

func (a *App) dropLive(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if c, ok := a.conns[id]; ok && c.cancel != nil {
		c.cancel()
	}
	delete(a.conns, id)
}
