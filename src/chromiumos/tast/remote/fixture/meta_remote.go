package fixture

import (
	"context"
	"time"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "metaRemote",
		Desc:            "Fixture for testing Tast's remote fixture support.",
		Contacts:        []string{"oka@chromium.org", "tast-owners@google.com"},
		Impl:            &metaRemoteFixt{},
		Vars:            []string{"meta.metaRemote.SetUpError", "meta.metaRemote.TearDownError"},
		SetUpTimeout:    1 * time.Minute,
		ResetTimeout:    1 * time.Minute,
		TearDownTimeout: 1 * time.Minute,
	})
}

type metaRemoteFixt struct{}

func (*metaRemoteFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("SetUp metaRemote")
	if x, ok := s.Var("meta.metaRemote.SetUpError"); ok {
		s.Error(x)
	}
	return nil
}

func (*metaRemoteFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("TearDown metaRemote")
	if x, ok := s.Var("meta.metaRemote.TearDownError"); ok {
		s.Error(x)
	}
}

func (*metaRemoteFixt) Reset(ctx context.Context) error                        { return nil }
func (*metaRemoteFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*metaRemoteFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
