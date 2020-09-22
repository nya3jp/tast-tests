package fixture

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "meta_remote",
		Desc:            "Fixture for testing Tast's remote fixture support.",
		Contacts:        []string{"oka@chromium.org", "tast-owners@google.com"},
		Impl:            &MetaRemote{},
		SetUpTimeout:    1 * time.Minute,
		ResetTimeout:    1 * time.Minute,
		TearDownTimeout: 1 * time.Minute,
	})
}

// MetaRemote fixture that supports only SetUp and TearDown.
type MetaRemote struct {
}

func (*MetaRemote) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("MetaRemote; SetUp")
	// The instance is not available from local tests.
	return nil
}

func (*MetaRemote) Reset(ctx context.Context) error {
	// Do nothing.
	return nil
}

func (*MetaRemote) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// Do nothing.
}

func (*MetaRemote) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// Do nothing.
}

func (*MetaRemote) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("MetaRemote; TearDown")
}
