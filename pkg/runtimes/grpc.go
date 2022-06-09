package runtimes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/proto"
	"github.com/vito/progrock"
	"github.com/vito/progrock/graph"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	Conn *grpc.ClientConn
	proto.RuntimeClient
}

func (client *Client) Resolve(ctx context.Context, ref bass.ThunkImageRef) (bass.ThunkImageRef, error) {
	ret := bass.ThunkImageRef{}

	p, err := ref.MarshalProto()
	if err != nil {
		return ret, err
	}

	r, err := client.RuntimeClient.Resolve(ctx, p.(*proto.ThunkImageRef))
	if err != nil {
		return ret, err
	}

	if err := ret.UnmarshalProto(r); err != nil {
		return ret, err
	}

	return ret, nil
}

func (client *Client) Run(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	p, err := thunk.MarshalProto()
	if err != nil {
		return err
	}

	r, err := client.RuntimeClient.Run(ctx, p.(*proto.Thunk))
	if err != nil {
		return err
	}

	recorder := progrock.RecorderFromContext(ctx)

	for {
		pov, err := r.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		switch x := pov.GetInner().(type) {
		case *proto.ProgressOrOutput_Progress:
			// TODO: just use protobuf for progrock

			status := &graph.SolveStatus{}

			for _, vtx := range x.Progress.Vertexes {
				inputs := []digest.Digest{}
				for _, i := range vtx.Inputs {
					inputs = append(inputs, digest.Digest(i))
				}

				status.Vertexes = append(status.Vertexes, &graph.Vertex{
					Digest:    digest.Digest(vtx.Digest),
					Inputs:    inputs,
					Name:      vtx.Name,
					Started:   timePtr(vtx.Started.AsTime()),
					Completed: timePtr(vtx.Completed.AsTime()),
					Cached:    vtx.Cached,
					Error:     vtx.GetError(),
				})
			}

			for _, log := range x.Progress.Logs {
				status.Logs = append(status.Logs, &graph.VertexLog{
					Vertex:    digest.Digest(log.Vertex),
					Stream:    int(log.Stream),
					Data:      log.Data,
					Timestamp: log.Timestamp.AsTime(),
				})
			}

			for _, st := range x.Progress.Statuses {
				status.Statuses = append(status.Statuses, &graph.VertexStatus{
					ID:        st.Id,
					Vertex:    digest.Digest(st.Vertex),
					Name:      st.Name,
					Total:     st.Total,
					Current:   st.Current,
					Timestamp: st.Timestamp.AsTime(),
					Started:   timePtr(st.Started.AsTime()),
					Completed: timePtr(st.Completed.AsTime()),
				})
			}

			recorder.Record(status)
		case *proto.ProgressOrOutput_Output:
			_, err := w.Write(x.Output)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unhandled stream message: %T", x)
		}
	}

	return nil
}

func (client *Client) Export(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	p, err := thunk.MarshalProto()
	if err != nil {
		return err
	}

	r, err := client.RuntimeClient.Export(ctx, p.(*proto.Thunk))
	if err != nil {
		return err
	}

	for {
		bytes, err := r.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		_, err = w.Write(bytes.GetData())
		if err != nil {
			return err
		}
	}

	return nil
}

func (client *Client) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	p, err := tp.MarshalProto()
	if err != nil {
		return err
	}

	r, err := client.RuntimeClient.ExportPath(ctx, p.(*proto.ThunkPath))
	if err != nil {
		return err
	}

	for {
		bytes, err := r.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		_, err = w.Write(bytes.GetData())
		if err != nil {
			return err
		}
	}

	return nil
}

func (client *Client) Prune(context.Context, bass.PruneOpts) error {
	return fmt.Errorf("Prune unimplemented")
}

func (client *Client) Close() error {
	return client.Conn.Close()
}

type Server struct {
	bass.Runtime

	proto.UnimplementedRuntimeServer
}

func (srv *Server) Resolve(ctx context.Context, p *proto.ThunkImageRef) (*proto.ThunkImageRef, error) {
	ref := bass.ThunkImageRef{}

	err := ref.UnmarshalProto(p)
	if err != nil {
		return nil, err
	}

	r, err := srv.Runtime.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}

	ret, err := r.MarshalProto()
	if err != nil {
		return nil, err
	}

	return ret.(*proto.ThunkImageRef), err
}

func (srv *Server) Run(p *proto.Thunk, runSrv proto.Runtime_RunServer) error {
	thunk := bass.Thunk{}

	err := thunk.UnmarshalProto(p)
	if err != nil {
		return err
	}

	recorder := progrock.NewRecorder(runSrvRecorder{runSrv})
	ctx := progrock.RecorderToContext(context.Background(), recorder)

	return srv.Runtime.Run(ctx, runSrvWriter{runSrv}, thunk)
}

func (srv *Server) Export(p *proto.Thunk, exportSrv proto.Runtime_ExportServer) error {
	thunk := bass.Thunk{}

	err := thunk.UnmarshalProto(p)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return srv.Runtime.Export(ctx, runSrvBytesWriter{exportSrv}, thunk)
}

func (srv *Server) ExportPath(p *proto.ThunkPath, exportSrv proto.Runtime_ExportPathServer) error {
	tp := bass.ThunkPath{}

	err := tp.UnmarshalProto(p)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return srv.Runtime.ExportPath(ctx, runSrvBytesWriter{exportSrv}, tp)
}

type runSrvRecorder struct {
	runSrv proto.Runtime_RunServer
}

func (w runSrvRecorder) WriteStatus(status *graph.SolveStatus) {
	prog := &proto.Progress{}

	for _, vtx := range status.Vertexes {
		inputs := []string{}
		for _, i := range vtx.Inputs {
			inputs = append(inputs, i.String())
		}

		p := &proto.Vertex{
			Digest: vtx.Digest.String(),
			Inputs: inputs,
			Name:   vtx.Name,
			Cached: vtx.Cached,
		}

		if vtx.Started != nil {
			p.Started = timestamppb.New(*vtx.Started)
		}

		if vtx.Completed != nil {
			p.Completed = timestamppb.New(*vtx.Completed)
		}

		if vtx.Error != "" {
			p.Error = &vtx.Error
		}

		prog.Vertexes = append(prog.Vertexes, p)
	}

	for _, log := range status.Logs {
		prog.Logs = append(prog.Logs, &proto.VertexLog{
			Vertex:    log.Vertex.String(),
			Stream:    int64(log.Stream),
			Data:      log.Data,
			Timestamp: timestamppb.New(log.Timestamp),
		})
	}

	for _, st := range status.Statuses {
		p := &proto.VertexStatus{
			Id:        st.ID,
			Vertex:    st.Vertex.String(),
			Name:      st.Name,
			Total:     st.Total,
			Current:   st.Current,
			Timestamp: timestamppb.New(st.Timestamp),
		}

		if st.Started != nil {
			p.Started = timestamppb.New(*st.Started)
		}

		if st.Completed != nil {
			p.Completed = timestamppb.New(*st.Completed)
		}

		prog.Statuses = append(prog.Statuses, p)
	}

	w.runSrv.Send(&proto.ProgressOrOutput{
		Inner: &proto.ProgressOrOutput_Progress{
			Progress: prog,
		},
	})
}

func (w runSrvRecorder) Close() {}

type runSrvWriter struct {
	runSrv proto.Runtime_RunServer
}

func (w runSrvWriter) Write(p []byte) (int, error) {
	err := w.runSrv.Send(&proto.ProgressOrOutput{
		Inner: &proto.ProgressOrOutput_Output{
			Output: p,
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}

type sendBytesServer interface {
	Send(*proto.Bytes) error
}

type runSrvBytesWriter struct {
	runSrv sendBytesServer
}

func (w runSrvBytesWriter) Write(p []byte) (int, error) {
	err := w.runSrv.Send(&proto.Bytes{Data: p})
	if err != nil {
		return 0, err
	}

	return len(p), nil
}
