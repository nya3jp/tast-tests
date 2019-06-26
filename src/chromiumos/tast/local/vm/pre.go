package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

type CrostiniPre struct {
	Chrome      *chrome.Chrome
	TestApiConn *chrome.Conn
	Container   *Container
}

func CrostiniStarted() testing.Precondition { return crostiniStartedPre }

var crostiniStartedPre = &preImpl{
	name:    "crostini_started",
	timeout: chrome.LoginTimeout + 10*time.Minute,
}

type preImpl struct {
	name    string
	timeout time.Duration
	cr      *chrome.Chrome
	tconn   *chrome.Conn
	cont    *Container
}

func (p *preImpl) String() string { return p.name }

func (p *preImpl) Timeout() time.Duration { return p.timeout }

func (p *preImpl) Prepare(ctx context.Context, s *testing.State) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	if p.cont != nil {
		// TODO(hollingum): sanity checks on the incoming state, see local/arc/pre.go.
		return p.buildCrostiniPre(ctx)
	}

	var err error
	if p.cr, err = chrome.New(ctx); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	s.Log("Enabling Crostini preference setting")
	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = EnableCrostini(ctx, p.tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}
	s.Log("Setting up component ", StagingComponent)
	if err = SetUpComponent(ctx, StagingComponent); err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")
	if p.cont, err = CreateDefaultContainer(ctx, s.OutDir(), p.cr.User(), StagingImageServer, ""); err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}

	locked = true
	chrome.Lock()

	// TODO(hollingum): cleanup code for if the precondition fails. See local/arc/pre.go.

	return p.buildCrostiniPre(ctx)

}

func (p *preImpl) Close(ctx context.Context, s *testing.State) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	locked = false
	chrome.Unlock()

	if err := p.cont.DumpLog(ctx, s.OutDir()); err != nil {
		s.Error("Failure dumping container log: ", err)
	}
	StopConcierge(ctx)
	UnmountComponent(ctx)
	p.cr.Close(ctx)
}

func (p *preImpl) buildCrostiniPre(ctx context.Context) CrostiniPre {
	p.cr.ResetState(ctx)
	return CrostiniPre{p.cr, p.tconn, p.cont}
}
