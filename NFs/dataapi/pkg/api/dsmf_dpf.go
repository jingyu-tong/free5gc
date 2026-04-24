package api

import (
	"context"

	"google.golang.org/grpc"
)

const (
	dpfControlServiceName            = "net.framework.dsmf_dpf.v1.DpfControlService"
	dsmfProcessingCallbackServiceName = "net.framework.dsmf_dpf.v1.DsmfProcessingCallbackService"
)

type DpfControlServiceClient interface {
	SubmitProcessingTask(ctx context.Context, in *SubmitProcessingTaskRequest, opts ...grpc.CallOption) (*SubmitProcessingTaskResponse, error)
}

type dpfControlServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDpfControlServiceClient(cc grpc.ClientConnInterface) DpfControlServiceClient {
	return &dpfControlServiceClient{cc: cc}
}

func (c *dpfControlServiceClient) SubmitProcessingTask(ctx context.Context, in *SubmitProcessingTaskRequest, opts ...grpc.CallOption) (*SubmitProcessingTaskResponse, error) {
	out := new(SubmitProcessingTaskResponse)
	err := c.cc.Invoke(ctx, "/"+dpfControlServiceName+"/SubmitProcessingTask", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type DpfControlServiceServer interface {
	SubmitProcessingTask(context.Context, *SubmitProcessingTaskRequest) (*SubmitProcessingTaskResponse, error)
}

func RegisterDpfControlServiceServer(s grpc.ServiceRegistrar, srv DpfControlServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: dpfControlServiceName,
		HandlerType: (*DpfControlServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "SubmitProcessingTask",
				Handler: func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(SubmitProcessingTaskRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return server.(DpfControlServiceServer).SubmitProcessingTask(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     server,
						FullMethod: "/" + dpfControlServiceName + "/SubmitProcessingTask",
					}
					handler := func(inner context.Context, req any) (any, error) {
						return server.(DpfControlServiceServer).SubmitProcessingTask(inner, req.(*SubmitProcessingTaskRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
	}, srv)
}

type DsmfProcessingCallbackServiceClient interface {
	ReportProcessingStatus(ctx context.Context, in *ProcessingStatusReport, opts ...grpc.CallOption) (*ProcessingStatusAck, error)
}

type dsmfProcessingCallbackServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDsmfProcessingCallbackServiceClient(cc grpc.ClientConnInterface) DsmfProcessingCallbackServiceClient {
	return &dsmfProcessingCallbackServiceClient{cc: cc}
}

func (c *dsmfProcessingCallbackServiceClient) ReportProcessingStatus(ctx context.Context, in *ProcessingStatusReport, opts ...grpc.CallOption) (*ProcessingStatusAck, error) {
	out := new(ProcessingStatusAck)
	err := c.cc.Invoke(ctx, "/"+dsmfProcessingCallbackServiceName+"/ReportProcessingStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type DsmfProcessingCallbackServiceServer interface {
	ReportProcessingStatus(context.Context, *ProcessingStatusReport) (*ProcessingStatusAck, error)
}

func RegisterDsmfProcessingCallbackServiceServer(s grpc.ServiceRegistrar, srv DsmfProcessingCallbackServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: dsmfProcessingCallbackServiceName,
		HandlerType: (*DsmfProcessingCallbackServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "ReportProcessingStatus",
				Handler: func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(ProcessingStatusReport)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return server.(DsmfProcessingCallbackServiceServer).ReportProcessingStatus(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     server,
						FullMethod: "/" + dsmfProcessingCallbackServiceName + "/ReportProcessingStatus",
					}
					handler := func(inner context.Context, req any) (any, error) {
						return server.(DsmfProcessingCallbackServiceServer).ReportProcessingStatus(inner, req.(*ProcessingStatusReport))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
	}, srv)
}
