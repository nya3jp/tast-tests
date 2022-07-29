package memoryuser

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"context"
	"time"
)

type CanaryType int64

const (
	Tab CanaryType = iota
	App
)

type Canary interface {
	Run(ctx context.Context) error
	Close(ctx context.Context)
	StillAlive(ctx context.Context) bool
	String() string
}

type TabCanary struct {
	cr     *chrome.Chrome
	server *MemoryStressServer
	unit   *MemoryStressUnit
}

type AppCanary struct {
	a     *arc.ARC
	tconn *chrome.TestConn
	unit  *ArcLifecycleUnit
}

func (c *TabCanary) Run(ctx context.Context) error {
	return c.unit.Run(ctx, c.cr)
}

func (c *TabCanary) Close(ctx context.Context) {
	c.server.Close()
	c.unit.Close(ctx, c.cr)
}

func (c *TabCanary) StillAlive(ctx context.Context) bool {
	return c.unit.StillAlive(ctx, c.cr)
}

func (c *TabCanary) String() string {
	return "Tab"
}

func (c *AppCanary) Run(ctx context.Context) error {
	return c.unit.Run(ctx, c.a, c.tconn)
}

func (c *AppCanary) Close(ctx context.Context) {
	c.unit.Close(ctx, c.a)
}

func (c *AppCanary) StillAlive(ctx context.Context) bool {
	return c.unit.StillAlive(ctx, c.a)
}

func (c *AppCanary) String() string {
	return "App"
}

func NewCanary(t CanaryType, ctx context.Context, allocMiB int, ratio float32, s *testing.State, cr *chrome.Chrome, a *arc.ARC) (Canary, error) {
	switch t {
	case Tab:
		server := NewMemoryStressServer(s.DataFileSystem())
		unit := server.NewMemoryStressUnit(50, 0.67, 2*time.Second)
		return &TabCanary{cr, server, unit}, nil
	case App:
		if err := InstallArcLifecycleTestApps(ctx, a, 1); err != nil {
			return nil, errors.Wrap(err, "failed to install the test app")
		}
		unit := NewArcLifecycleUnit(0, 50, 0.67, nil, true)
		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a test API connection")
		}
		return &AppCanary{a, tconn, unit}, nil
	default:
		//Never reaches
		return nil, nil
	}
}
