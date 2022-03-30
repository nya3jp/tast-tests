package example

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"

	"context"
	"time"
)

type failingFixt struct {
}

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "failingfixture",
		Desc:            "Fixture failing reset",
		Contacts:        []string{"vsavu@google.com", "chromeos-commercial-remote-management@google.com"},
		Impl:            &failingFixt{},
		SetUpTimeout:    8 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		ResetTimeout:    15 * time.Second,
	})
}

var started = false

func (e *failingFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if started {
		s.Error("Already started")
	}

	started = true
	return nil
}
func (e *failingFixt) TearDown(ctx context.Context, s *testing.FixtState) {}
func (*failingFixt) Reset(ctx context.Context) error {
	return errors.New("forced err")
}
func (*failingFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*failingFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
