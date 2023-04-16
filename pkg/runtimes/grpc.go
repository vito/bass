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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	Conn *grpc.ClientConn
	proto.RuntimeClient
}

var _ bass.Runtime = &Client{}

const GRPCName = "grpc"

func init() {
	RegisterRuntime(GRPCName, NewClient)
}

type ClientConfig struct {
	Target string `json:"target"`
}

func NewClient(ctx context.Context, _ bass.RuntimePool, cfg *bass.Scope) (bass.Runtime, error) {
	var config ClientConfig
	if cfg != nil {
		if err := cfg.Decode(&config); err != nil {
			return nil, fmt.Errorf("buildkit runtime config: %w", err)
		}
	}

	conn, err := grpc.Dial(config.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Client{
		Conn:          conn,
		RuntimeClient: proto.NewRuntimeClient(conn),
	}, nil
}

func (client *Client) Resolve(ctx context.Context, ref bass.ImageRef) (bass.Thunk, error) {
	p, err := ref.MarshalProto()
	if err != nil {
		return bass.Thunk{}, err
	}

	r, err := client.RuntimeClient.Resolve(ctx, p.(*proto.ImageRef))
	if err != nil {
		return bass.Thunk{}, err
	}

	ret := bass.Thunk{}
	if err := ret.UnmarshalProto(r); err != nil {
		return bass.Thunk{}, err
	}

	return ret, nil
}

func (client *Client) Run(ctx context.Context, thunk bass.Thunk) error {
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
		case *proto.RunResponse_Progress:
			recorder.Record(progressToStatus(x.Progress))

		default:
			return fmt.Errorf("unhandled stream message: %T", x)
		}
	}

	return nil
}

func (client *Client) Read(ctx context.Context, w io.Writer, thunk bass.Thunk) error {
	p, err := thunk.MarshalProto()
	if err != nil {
		return err
	}

	r, err := client.RuntimeClient.Read(ctx, p.(*proto.Thunk))
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
		case *proto.ReadResponse_Progress:
			recorder.Record(progressToStatus(x.Progress))

		case *proto.ReadResponse_Output:
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

	stream, err := client.RuntimeClient.Export(ctx, p.(*proto.Thunk))
	if err != nil {
		return err
	}

	recorder := progrock.RecorderFromContext(ctx)

	for {
		pod, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		switch x := pod.GetInner().(type) {
		case *proto.ExportResponse_Progress:
			recorder.Record(progressToStatus(x.Progress))

		case *proto.ExportResponse_Data:
			_, err = w.Write(x.Data)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unhandled stream message: %T", x)
		}
	}

	return nil
}

func (client *Client) Publish(ctx context.Context, ref bass.ImageRef, thunk bass.Thunk) (bass.ImageRef, error) {
	ret := bass.ImageRef{}

	t, err := ref.MarshalProto()
	if err != nil {
		return ret, err
	}

	r, err := ref.MarshalProto()
	if err != nil {
		return ret, err
	}

	stream, err := client.RuntimeClient.Publish(ctx, &proto.PublishRequest{
		Ref:   r.(*proto.ImageRef),
		Thunk: t.(*proto.Thunk),
	})
	if err != nil {
		return ref, err
	}

	recorder := progrock.RecorderFromContext(ctx)

	for {
		pov, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return ret, err
		}

		switch x := pov.GetInner().(type) {
		case *proto.PublishResponse_Progress:
			recorder.Record(progressToStatus(x.Progress))

		case *proto.PublishResponse_Published:
			err := ret.UnmarshalProto(x.Published)
			if err != nil {
				return ret, err
			}

		default:
			return ret, fmt.Errorf("unhandled stream message: %T", x)
		}
	}

	return ret, nil
}

func (client *Client) ExportPath(ctx context.Context, w io.Writer, tp bass.ThunkPath) error {
	p, err := tp.MarshalProto()
	if err != nil {
		return err
	}

	stream, err := client.RuntimeClient.ExportPath(ctx, p.(*proto.ThunkPath))
	if err != nil {
		return err
	}

	recorder := progrock.RecorderFromContext(ctx)

	for {
		pod, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		switch x := pod.GetInner().(type) {
		case *proto.ExportResponse_Progress:
			recorder.Record(progressToStatus(x.Progress))

		case *proto.ExportResponse_Data:
			_, err = w.Write(x.Data)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unhandled stream message: %T", x)
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
	Context context.Context
	Runtime bass.Runtime

	proto.UnimplementedRuntimeServer
}

func (srv *Server) Resolve(ctx context.Context, p *proto.ImageRef) (*proto.Thunk, error) {
	ref := bass.ImageRef{}

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

	return ret.(*proto.Thunk), err
}

func (srv *Server) Run(p *proto.Thunk, runSrv proto.Runtime_RunServer) error {
	thunk := bass.Thunk{}

	err := thunk.UnmarshalProto(p)
	if err != nil {
		return err
	}

	recorder := progrock.NewRecorder(runSrvRecorder{runSrv})
	ctx := progrock.RecorderToContext(srv.Context, recorder)

	return srv.Runtime.Run(ctx, thunk)
}

func (srv *Server) Read(p *proto.Thunk, readSrv proto.Runtime_ReadServer) error {
	thunk := bass.Thunk{}

	err := thunk.UnmarshalProto(p)
	if err != nil {
		return err
	}

	recorder := progrock.NewRecorder(readSrvRecorder{readSrv})
	ctx := progrock.RecorderToContext(srv.Context, recorder)

	return srv.Runtime.Read(ctx, readSrvWriter{readSrv}, thunk)
}

func (srv *Server) Export(p *proto.Thunk, exportSrv proto.Runtime_ExportServer) error {
	thunk := bass.Thunk{}

	err := thunk.UnmarshalProto(p)
	if err != nil {
		return err
	}

	recorder := progrock.NewRecorder(exportSrvRecorder{exportSrv})
	ctx := progrock.RecorderToContext(srv.Context, recorder)

	return srv.Runtime.Export(ctx, exportSrvWriter{exportSrv}, thunk)
}

func (srv *Server) Publish(p *proto.PublishRequest, pubSrv proto.Runtime_PublishServer) error {
	thunk := bass.Thunk{}
	if err := thunk.UnmarshalProto(p.GetThunk()); err != nil {
		return err
	}

	ref := bass.ImageRef{}
	if err := thunk.UnmarshalProto(p.GetRef()); err != nil {
		return err
	}

	recorder := progrock.NewRecorder(publishSrvRecorder{pubSrv})
	ctx := progrock.RecorderToContext(srv.Context, recorder)

	ref, err := srv.Runtime.Publish(ctx, ref, thunk)
	if err != nil {
		return err
	}

	pRef, err := ref.MarshalProto()
	if err != nil {
		return err
	}

	return pubSrv.Send(&proto.PublishResponse{
		Inner: &proto.PublishResponse_Published{
			Published: pRef.(*proto.ImageRef),
		},
	})
}

func (srv *Server) ExportPath(p *proto.ThunkPath, exportSrv proto.Runtime_ExportPathServer) error {
	tp := bass.ThunkPath{}

	err := tp.UnmarshalProto(p)
	if err != nil {
		return err
	}

	recorder := progrock.NewRecorder(exportSrvRecorder{exportSrv})
	ctx := progrock.RecorderToContext(srv.Context, recorder)

	return srv.Runtime.ExportPath(ctx, exportSrvWriter{exportSrv}, tp)
}

type runSrvRecorder struct {
	srv proto.Runtime_RunServer
}

func (w runSrvRecorder) WriteStatus(status *graph.SolveStatus) {
	w.srv.Send(&proto.RunResponse{
		Inner: &proto.RunResponse_Progress{
			Progress: statusToProgress(status),
		},
	})
}

func (w runSrvRecorder) Close() {}

type readSrvRecorder struct {
	readSrv proto.Runtime_ReadServer
}

func (w readSrvRecorder) WriteStatus(status *graph.SolveStatus) {
	w.readSrv.Send(&proto.ReadResponse{
		Inner: &proto.ReadResponse_Progress{
			Progress: statusToProgress(status),
		},
	})
}

func (w readSrvRecorder) Close() {}

type readSrvWriter struct {
	runSrv proto.Runtime_ReadServer
}

func (w readSrvWriter) Write(p []byte) (int, error) {
	err := w.runSrv.Send(&proto.ReadResponse{
		Inner: &proto.ReadResponse_Output{
			Output: p,
		},
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

type publishSrvRecorder struct {
	publishSrv proto.Runtime_PublishServer
}

func (w publishSrvRecorder) WriteStatus(status *graph.SolveStatus) {
	w.publishSrv.Send(&proto.PublishResponse{
		Inner: &proto.PublishResponse_Progress{
			Progress: statusToProgress(status),
		},
	})
}

func (w publishSrvRecorder) Close() {}

func timePtr(t time.Time) *time.Time {
	return &t
}

type exportResponseSrv interface {
	Send(*proto.ExportResponse) error
}

type exportSrvWriter struct {
	srv exportResponseSrv
}

func (w exportSrvWriter) Write(p []byte) (int, error) {
	err := w.srv.Send(&proto.ExportResponse{
		Inner: &proto.ExportResponse_Data{
			Data: p,
		},
	})
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

type exportSrvRecorder struct {
	srv exportResponseSrv
}

func (w exportSrvRecorder) WriteStatus(status *graph.SolveStatus) {
	w.srv.Send(&proto.ExportResponse{
		Inner: &proto.ExportResponse_Progress{
			Progress: statusToProgress(status),
		},
	})
}

func (w exportSrvRecorder) Close() {}

func statusToProgress(status *graph.SolveStatus) *proto.Progress {
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

	for _, vertexLog := range status.Logs {
		prog.Logs = append(prog.Logs, &proto.VertexLog{
			Vertex:    vertexLog.Vertex.String(),
			Stream:    int64(vertexLog.Stream),
			Data:      vertexLog.Data,
			Timestamp: timestamppb.New(vertexLog.Timestamp),
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

	return prog
}

// TODO: just use protobuf for progrock
func progressToStatus(progress *proto.Progress) *graph.SolveStatus {
	status := &graph.SolveStatus{}

	for _, vtx := range progress.Vertexes {
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

	for _, log := range progress.Logs {
		status.Logs = append(status.Logs, &graph.VertexLog{
			Vertex:    digest.Digest(log.Vertex),
			Stream:    int(log.Stream),
			Data:      log.Data,
			Timestamp: log.Timestamp.AsTime(),
		})
	}

	for _, st := range progress.Statuses {
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

	return status
}
