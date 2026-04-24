package api

import (
	"context"

	"google.golang.org/grpc"
)

const (
	dsfControlServiceName         = "net.framework.dsmf_dsf.v1.DsfControlService"
	dsmfStorageCallbackServiceName = "net.framework.dsmf_dsf.v1.DsmfStorageCallbackService"
)

type DsfControlServiceClient interface {
	SubmitStorageTask(ctx context.Context, in *SubmitStorageTaskRequest, opts ...grpc.CallOption) (*SubmitStorageTaskResponse, error)
}

type dsfControlServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDsfControlServiceClient(cc grpc.ClientConnInterface) DsfControlServiceClient {
	return &dsfControlServiceClient{cc: cc}
}

func (c *dsfControlServiceClient) SubmitStorageTask(ctx context.Context, in *SubmitStorageTaskRequest, opts ...grpc.CallOption) (*SubmitStorageTaskResponse, error) {
	out := new(SubmitStorageTaskResponse)
	err := c.cc.Invoke(ctx, "/"+dsfControlServiceName+"/SubmitStorageTask", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type DsfControlServiceServer interface {
	SubmitStorageTask(context.Context, *SubmitStorageTaskRequest) (*SubmitStorageTaskResponse, error)
}

func RegisterDsfControlServiceServer(s grpc.ServiceRegistrar, srv DsfControlServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: dsfControlServiceName,
		HandlerType: (*DsfControlServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "SubmitStorageTask",
				Handler: func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(SubmitStorageTaskRequest)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return server.(DsfControlServiceServer).SubmitStorageTask(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     server,
						FullMethod: "/" + dsfControlServiceName + "/SubmitStorageTask",
					}
					handler := func(inner context.Context, req any) (any, error) {
						return server.(DsfControlServiceServer).SubmitStorageTask(inner, req.(*SubmitStorageTaskRequest))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
	}, srv)
}

type DsmfStorageCallbackServiceClient interface {
	ReportStorageStatus(ctx context.Context, in *StorageStatusReport, opts ...grpc.CallOption) (*StorageStatusAck, error)
}

type dsmfStorageCallbackServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDsmfStorageCallbackServiceClient(cc grpc.ClientConnInterface) DsmfStorageCallbackServiceClient {
	return &dsmfStorageCallbackServiceClient{cc: cc}
}

func (c *dsmfStorageCallbackServiceClient) ReportStorageStatus(ctx context.Context, in *StorageStatusReport, opts ...grpc.CallOption) (*StorageStatusAck, error) {
	out := new(StorageStatusAck)
	err := c.cc.Invoke(ctx, "/"+dsmfStorageCallbackServiceName+"/ReportStorageStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type DsmfStorageCallbackServiceServer interface {
	ReportStorageStatus(context.Context, *StorageStatusReport) (*StorageStatusAck, error)
}

func RegisterDsmfStorageCallbackServiceServer(s grpc.ServiceRegistrar, srv DsmfStorageCallbackServiceServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: dsmfStorageCallbackServiceName,
		HandlerType: (*DsmfStorageCallbackServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "ReportStorageStatus",
				Handler: func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := new(StorageStatusReport)
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return server.(DsmfStorageCallbackServiceServer).ReportStorageStatus(ctx, in)
					}
					info := &grpc.UnaryServerInfo{
						Server:     server,
						FullMethod: "/" + dsmfStorageCallbackServiceName + "/ReportStorageStatus",
					}
					handler := func(inner context.Context, req any) (any, error) {
						return server.(DsmfStorageCallbackServiceServer).ReportStorageStatus(inner, req.(*StorageStatusReport))
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
	}, srv)
}
