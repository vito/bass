// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.21.12
// source: runtime.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Runtime_Resolve_FullMethodName    = "/bass.Runtime/Resolve"
	Runtime_Run_FullMethodName        = "/bass.Runtime/Run"
	Runtime_Read_FullMethodName       = "/bass.Runtime/Read"
	Runtime_Export_FullMethodName     = "/bass.Runtime/Export"
	Runtime_Publish_FullMethodName    = "/bass.Runtime/Publish"
	Runtime_ExportPath_FullMethodName = "/bass.Runtime/ExportPath"
)

// RuntimeClient is the client API for Runtime service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type RuntimeClient interface {
	Resolve(ctx context.Context, in *ImageRef, opts ...grpc.CallOption) (*ImageRef, error)
	Run(ctx context.Context, in *Thunk, opts ...grpc.CallOption) (Runtime_RunClient, error)
	Read(ctx context.Context, in *Thunk, opts ...grpc.CallOption) (Runtime_ReadClient, error)
	Export(ctx context.Context, in *Thunk, opts ...grpc.CallOption) (Runtime_ExportClient, error)
	Publish(ctx context.Context, in *PublishRequest, opts ...grpc.CallOption) (Runtime_PublishClient, error)
	ExportPath(ctx context.Context, in *ThunkPath, opts ...grpc.CallOption) (Runtime_ExportPathClient, error)
}

type runtimeClient struct {
	cc grpc.ClientConnInterface
}

func NewRuntimeClient(cc grpc.ClientConnInterface) RuntimeClient {
	return &runtimeClient{cc}
}

func (c *runtimeClient) Resolve(ctx context.Context, in *ImageRef, opts ...grpc.CallOption) (*ImageRef, error) {
	out := new(ImageRef)
	err := c.cc.Invoke(ctx, Runtime_Resolve_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *runtimeClient) Run(ctx context.Context, in *Thunk, opts ...grpc.CallOption) (Runtime_RunClient, error) {
	stream, err := c.cc.NewStream(ctx, &Runtime_ServiceDesc.Streams[0], Runtime_Run_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &runtimeRunClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Runtime_RunClient interface {
	Recv() (*RunResponse, error)
	grpc.ClientStream
}

type runtimeRunClient struct {
	grpc.ClientStream
}

func (x *runtimeRunClient) Recv() (*RunResponse, error) {
	m := new(RunResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *runtimeClient) Read(ctx context.Context, in *Thunk, opts ...grpc.CallOption) (Runtime_ReadClient, error) {
	stream, err := c.cc.NewStream(ctx, &Runtime_ServiceDesc.Streams[1], Runtime_Read_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &runtimeReadClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Runtime_ReadClient interface {
	Recv() (*ReadResponse, error)
	grpc.ClientStream
}

type runtimeReadClient struct {
	grpc.ClientStream
}

func (x *runtimeReadClient) Recv() (*ReadResponse, error) {
	m := new(ReadResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *runtimeClient) Export(ctx context.Context, in *Thunk, opts ...grpc.CallOption) (Runtime_ExportClient, error) {
	stream, err := c.cc.NewStream(ctx, &Runtime_ServiceDesc.Streams[2], Runtime_Export_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &runtimeExportClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Runtime_ExportClient interface {
	Recv() (*Bytes, error)
	grpc.ClientStream
}

type runtimeExportClient struct {
	grpc.ClientStream
}

func (x *runtimeExportClient) Recv() (*Bytes, error) {
	m := new(Bytes)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *runtimeClient) Publish(ctx context.Context, in *PublishRequest, opts ...grpc.CallOption) (Runtime_PublishClient, error) {
	stream, err := c.cc.NewStream(ctx, &Runtime_ServiceDesc.Streams[3], Runtime_Publish_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &runtimePublishClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Runtime_PublishClient interface {
	Recv() (*PublishResponse, error)
	grpc.ClientStream
}

type runtimePublishClient struct {
	grpc.ClientStream
}

func (x *runtimePublishClient) Recv() (*PublishResponse, error) {
	m := new(PublishResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *runtimeClient) ExportPath(ctx context.Context, in *ThunkPath, opts ...grpc.CallOption) (Runtime_ExportPathClient, error) {
	stream, err := c.cc.NewStream(ctx, &Runtime_ServiceDesc.Streams[4], Runtime_ExportPath_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &runtimeExportPathClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Runtime_ExportPathClient interface {
	Recv() (*Bytes, error)
	grpc.ClientStream
}

type runtimeExportPathClient struct {
	grpc.ClientStream
}

func (x *runtimeExportPathClient) Recv() (*Bytes, error) {
	m := new(Bytes)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// RuntimeServer is the server API for Runtime service.
// All implementations must embed UnimplementedRuntimeServer
// for forward compatibility
type RuntimeServer interface {
	Resolve(context.Context, *ImageRef) (*ImageRef, error)
	Run(*Thunk, Runtime_RunServer) error
	Read(*Thunk, Runtime_ReadServer) error
	Export(*Thunk, Runtime_ExportServer) error
	Publish(*PublishRequest, Runtime_PublishServer) error
	ExportPath(*ThunkPath, Runtime_ExportPathServer) error
	mustEmbedUnimplementedRuntimeServer()
}

// UnimplementedRuntimeServer must be embedded to have forward compatible implementations.
type UnimplementedRuntimeServer struct {
}

func (UnimplementedRuntimeServer) Resolve(context.Context, *ImageRef) (*ImageRef, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Resolve not implemented")
}
func (UnimplementedRuntimeServer) Run(*Thunk, Runtime_RunServer) error {
	return status.Errorf(codes.Unimplemented, "method Run not implemented")
}
func (UnimplementedRuntimeServer) Read(*Thunk, Runtime_ReadServer) error {
	return status.Errorf(codes.Unimplemented, "method Read not implemented")
}
func (UnimplementedRuntimeServer) Export(*Thunk, Runtime_ExportServer) error {
	return status.Errorf(codes.Unimplemented, "method Export not implemented")
}
func (UnimplementedRuntimeServer) Publish(*PublishRequest, Runtime_PublishServer) error {
	return status.Errorf(codes.Unimplemented, "method Publish not implemented")
}
func (UnimplementedRuntimeServer) ExportPath(*ThunkPath, Runtime_ExportPathServer) error {
	return status.Errorf(codes.Unimplemented, "method ExportPath not implemented")
}
func (UnimplementedRuntimeServer) mustEmbedUnimplementedRuntimeServer() {}

// UnsafeRuntimeServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to RuntimeServer will
// result in compilation errors.
type UnsafeRuntimeServer interface {
	mustEmbedUnimplementedRuntimeServer()
}

func RegisterRuntimeServer(s grpc.ServiceRegistrar, srv RuntimeServer) {
	s.RegisterService(&Runtime_ServiceDesc, srv)
}

func _Runtime_Resolve_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ImageRef)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RuntimeServer).Resolve(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Runtime_Resolve_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RuntimeServer).Resolve(ctx, req.(*ImageRef))
	}
	return interceptor(ctx, in, info, handler)
}

func _Runtime_Run_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Thunk)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RuntimeServer).Run(m, &runtimeRunServer{stream})
}

type Runtime_RunServer interface {
	Send(*RunResponse) error
	grpc.ServerStream
}

type runtimeRunServer struct {
	grpc.ServerStream
}

func (x *runtimeRunServer) Send(m *RunResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _Runtime_Read_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Thunk)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RuntimeServer).Read(m, &runtimeReadServer{stream})
}

type Runtime_ReadServer interface {
	Send(*ReadResponse) error
	grpc.ServerStream
}

type runtimeReadServer struct {
	grpc.ServerStream
}

func (x *runtimeReadServer) Send(m *ReadResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _Runtime_Export_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Thunk)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RuntimeServer).Export(m, &runtimeExportServer{stream})
}

type Runtime_ExportServer interface {
	Send(*Bytes) error
	grpc.ServerStream
}

type runtimeExportServer struct {
	grpc.ServerStream
}

func (x *runtimeExportServer) Send(m *Bytes) error {
	return x.ServerStream.SendMsg(m)
}

func _Runtime_Publish_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(PublishRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RuntimeServer).Publish(m, &runtimePublishServer{stream})
}

type Runtime_PublishServer interface {
	Send(*PublishResponse) error
	grpc.ServerStream
}

type runtimePublishServer struct {
	grpc.ServerStream
}

func (x *runtimePublishServer) Send(m *PublishResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _Runtime_ExportPath_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ThunkPath)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(RuntimeServer).ExportPath(m, &runtimeExportPathServer{stream})
}

type Runtime_ExportPathServer interface {
	Send(*Bytes) error
	grpc.ServerStream
}

type runtimeExportPathServer struct {
	grpc.ServerStream
}

func (x *runtimeExportPathServer) Send(m *Bytes) error {
	return x.ServerStream.SendMsg(m)
}

// Runtime_ServiceDesc is the grpc.ServiceDesc for Runtime service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Runtime_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "bass.Runtime",
	HandlerType: (*RuntimeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Resolve",
			Handler:    _Runtime_Resolve_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Run",
			Handler:       _Runtime_Run_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Read",
			Handler:       _Runtime_Read_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Export",
			Handler:       _Runtime_Export_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Publish",
			Handler:       _Runtime_Publish_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ExportPath",
			Handler:       _Runtime_ExportPath_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "runtime.proto",
}
