package service

import (
	"io/ioutil"

	"github.com/golang/protobuf/ptypes"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	svcpb "chromiumos/tast/service/cros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Data: nil,
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			svcpb.RegisterFileSystemServer(srv, &FileSystemImpl{s})
		},
	})
}

type FileSystemImpl struct {
	s *testing.ServiceState
}

func (fs *FileSystemImpl) ReadDir(ctx context.Context, req *svcpb.ReadDirRequest) (*svcpb.ReadDirReply, error) {
	fis, err := ioutil.ReadDir(req.Dir)
	if err != nil {
		return nil, err
	}

	var res svcpb.ReadDirReply
	for _, fi := range fis {
		ts, err := ptypes.TimestampProto(fi.ModTime())
		if err != nil {
			return nil, err
		}
		res.Files = append(res.Files, &svcpb.FileInfo{
			Name: fi.Name(),
			Size: uint64(fi.Size()),
			Mode: uint64(fi.Mode()),
			Modified: ts,
		})
	}
	return &res, nil
}
