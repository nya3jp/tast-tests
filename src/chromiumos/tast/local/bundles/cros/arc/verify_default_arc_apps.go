package arc

import(
        "context"
        "time"
        "chromiumos/tast/local/chrome"
        "chromiumos/tast/testing"
        "chromiumos/tast/local/apps"
        "chromiumos/tast/local/chrome/ash"
        "chromiumos/tast/local/arc"
)

func init() {
        testing.AddTest(&testing.Test{
                Func: VerifyDefaultArcApps,
                Desc: "Verifies Default arc apps",
                Contacts: []string{
                        "vkrishan@google.com",
                        "rohitbm@google.com",
                },
                Attr:         []string{"group:mainline", "informational"},
                Timeout:      5 * time.Minute,
                SoftwareDeps: []string{"chrome"},
                Params: []testing.Param{{
                        ExtraSoftwareDeps: []string{"android_p"},
                        Val:               []string{},
                }, {
                        Name:              "vm",
                        ExtraSoftwareDeps: []string{"android_vm"},
                        Val:               []string{"--enable-arcvm"},
                }},

        })
}

func VerifyDefaultArcApps(ctx context.Context, s *testing.State){

	extraArgs := s.Param().([]string)
        args := []string{"--arc-disable-app-sync", "--arc-disable-play-auto-install", "--arc-disable-locale-sync", "--arc-play-store-auto-update=off"}
        args = append(args, extraArgs...)

        cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.ExtraArgs(args...))
        if err != nil {
                s.Fatal("Failed to connect to Chrome: ", err)
        }
        defer cr.Close(ctx)

        a, err := arc.New(ctx, s.OutDir())
        if err != nil {
                s.Fatal("Failed to start ARC: ", err)
        }
        defer a.Close()

	tconn, err := cr.TestAPIConn(ctx)
        if err !=nil{
                s.Fatal("Failed to connect Test API: ", err)
        }
        
	// Searching for ARC++ default apps
        if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayStore.ID, 3*time.Minute); err != nil {
                s.Fatal("Failed to wait for installed app: ", err)
        }

        if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.Duo.ID, 3*time.Minute); err != nil {
                s.Fatal("Failed to wait for installed app: ", err)
        }

        if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayMusic.ID, 3*time.Minute); err != nil {
                s.Fatal("Failed to wait for installed app: ", err)
        }

        if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayBooks.ID, 3*time.Minute); err != nil {
                s.Fatal("Failed to wait for installed app: ", err)
        }

        if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayGames.ID, 3*time.Minute); err != nil {
                s.Fatal("Failed to wait for installed app: ", err)
        }

        if err := ash.WaitForChromeAppInstalled(ctx, tconn, apps.PlayMovies.ID, 3*time.Minute); err != nil {
                s.Fatal("Failed to wait for installed app: ", err)
        }
}


