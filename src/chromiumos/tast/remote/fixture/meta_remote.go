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
		SetUpTimeout:    1 * time.Minute,
		ResetTimeout:    1 * time.Minute,
		TearDownTimeout: 1 * time.Minute,
	})
}

// MetaRemoteFixtConfigLocation is the file name of the config file instructs
// how this fixture should behave. Meta tests should create the file under
// s.OutDir().
const MetaRemoteFixtConfigLocation = "meta_remote_fixt_config.json"

type MetaRemoteFixtConfig struct {
	SetUpErrors []string `json:"setUpErrors"`
}

type metaRemoteFixt struct{}

func (*metaRemoteFixt) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.OutDir()
	s.Log("metaRemote - SetUp")
	return nil
}

func (*metaRemoteFixt) TearDown(ctx context.Context, s *testing.FixtState) {
	s.Log("metaRemote - TearDown")
}

func (*metaRemoteFixt) Reset(ctx context.Context) error                        { return nil }
func (*metaRemoteFixt) PreTest(ctx context.Context, s *testing.FixtTestState)  {}
func (*metaRemoteFixt) PostTest(ctx context.Context, s *testing.FixtTestState) {}
