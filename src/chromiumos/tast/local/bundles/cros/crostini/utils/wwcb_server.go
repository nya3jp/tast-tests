package utils

import (
	"context"
	"fmt"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func GetServerFile(ctx context.Context, s *testing.State, storepath, filename string) error {

	s.Log("Get server file ")

	serverFile := fmt.Sprintf("%s/%s/%s", getWebUrl(s), "script_upload", filename)

	getFile := testexec.CommandContext(ctx, "wget", "-P", storepath, serverFile)

	_, err := getFile.Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrapf(err, "%q failed: ", shutil.EscapeSlice(getFile.Args))
	}

	return nil
}

func CopyFile(ctx context.Context, s *testing.State, localpath string) error {
	return nil
}
