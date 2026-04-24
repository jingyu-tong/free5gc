package api

import (
	"context"
	"io"

	"google.golang.org/grpc"
)

const resultTransferServiceName = "net.framework.dpf_dsf.v1.ResultTransferService"

type ResultTransferServiceClient interface {
	OpenTransfer(ctx context.Context, in *OpenTransferRequest, opts ...grpc.CallOption) (*OpenTransferResponse, error)
	PushResult(ctx context.Context, opts ...grpc.CallOption) (ResultTransferService_PushResultClient, error)
	CloseTransfer(ctx context.Context, in *CloseTransferRequest, opts ...grpc.CallOption) (*CloseTransferResponse, error)
}

type resultTransferServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewResultTransferServiceClient(cc grpc.ClientConnInterface) ResultTransferServiceClient {
	return &resultTransferServiceClient{cc: cc}
}

func (c *resultTransferServiceClient) OpenTransfer(ctx context.Context, in *OpenTransferRequest, opts ...grpc.CallOption) (*OpenTransferResponse, error) {
	out := new(OpenTransferResponse)
	err := c.cc.Invoke(ctx, "/"+resultTransferServiceName+"/OpenTransfer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *resultTransferServiceClient) PushResult(ctx context.Context, opts ...grpc.CallOption) (ResultTransferService_PushResultClient, error) {
	stream, err := c.cc.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "PushResult",
		ClientStreams: true,
	}, "/"+resultTransferServiceName+"/PushResult", opts...)
	if err != nil {
		return nil, err
	}
	return &resultTransferServicePushResultClient{ClientStream: stream}, nil
}

func (c *resultTransferServiceClient) CloseTransfer(ctx context.Context, in *CloseTransferRequest, opts ...grpc.CallOption) (*CloseTransferResponse, error) {
	out := new(CloseTransferResponse)
	err := c.cc.Invoke(ctx, "/"+resultTransferServiceName+"/CloseTransfer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type ResultTransferService_PushResultClient interface {
	Send(*ResultChunk) error
	CloseAndRecv() (*PushResultResponse, error)
	grpc.ClientStream
}

type resultTransferServicePushResultClient struct {
	grpc.ClientStream
}

func (c *resultTransferServicePushResultClient) Send(in *ResultChunk) error {
	return c.ClientStream.SendMsg(in)
}

func (c *resultTransferServicePushResultClient) CloseAndRecv() (*PushResultResponse, error) {
	if err := c.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	out := new(PushResultResponse)
	if err := c.ClientStream.RecvMsg(out); err != nil {
		return nil, err
	}
	return out, nil
}

type ResultTransferServiceServer interface {
	OpenTransfer(context.Context, *OpenTransferRequest) (*OpenTransferResponse, error)
	PushResult(ResultTransferService_PushResultServer) error
	CloseTransfer(context.Context, *CloseTransferRequest) (*CloseTransferResponse, error)
}

type ResultTransferService_PushResultServer interface {
	SendAndClose(*PushResultResponse) error
	Recv() (*ResultChunk, error)
	grpc.ServerStream
}

func RegisterResultTransferServiceServer(s grpc.ServiceRegistrar, srv ResultTransferServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: resultTransferServiceName,
		HandlerType: (*ResultTransferServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "OpenTransfer",
				Handler: func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(OpenTransferRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return server.(ResultTransferServiceServer).OpenTransfer(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     server,
						FullMethod: "/" + resultTransferServiceName + "/OpenTransfer",
					}
					handler := func(inner context.Context, req any) (any, error) {
						return server.(ResultTransferServiceServer).OpenTransfer(inner, req.(*OpenTransferRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
			{
				MethodName: "CloseTransfer",
				Handler: func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(CloseTransferRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return server.(ResultTransferServiceServer).CloseTransfer(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     server,
						FullMethod: "/" + resultTransferServiceName + "/CloseTransfer",
					}
					handler := func(inner context.Context, req any) (any, error) {
						return server.(ResultTransferServiceServer).CloseTransfer(inner, req.(*CloseTransferRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "PushResult",
				ClientStreams: true,
				Handler: func(server any, stream grpc.ServerStream) error {
					return server.(ResultTransferServiceServer).PushResult(&resultTransferServicePushResultServer{ServerStream: stream})
				},
			},
		},
	}, srv)
}

type resultTransferServicePushResultServer struct {
	grpc.ServerStream
}

func (s *resultTransferServicePushResultServer) SendAndClose(resp *PushResultResponse) error {
	return s.ServerStream.SendMsg(resp)
}

func (s *resultTransferServicePushResultServer) Recv() (*ResultChunk, error) {
	in := new(ResultChunk)
	err := s.ServerStream.RecvMsg(in)
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, err
	}
	return in, nil
}
