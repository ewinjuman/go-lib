package grpc

import (
	"context"
	"github.com/ewinjuman/go-lib/v2/constant"
	"time"

	"github.com/ewinjuman/go-lib/v2/appContext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Options struct {
	Address string        `json:"address"`
	Timeout time.Duration `json:"timeout"`
}

type RpcConnection struct {
	options    Options
	Connection *grpc.ClientConn
}

func (rpc *RpcConnection) CreateContext(parent context.Context, appCtx *appContext.AppContext) (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithTimeout(parent, rpc.options.Timeout*time.Second)
	ctx = context.WithValue(ctx, constant.AppContextKey, appCtx)
	md := metadata.New(map[string]string{"Request-Id": appCtx.RequestID})
	ctx = metadata.NewOutgoingContext(ctx, md)
	return
}

func New(options Options) (rpc *RpcConnection, err error) {
	connection, err := grpc.Dial(options.Address, grpc.WithInsecure(), grpc.WithUnaryInterceptor(clientInterceptor))
	if err != nil {
		return nil, err
	}

	rpc = &RpcConnection{
		Connection: connection,
		options:    options,
	}
	return
}

func clientInterceptor(ctx context.Context, method string, request interface{}, response interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	timeStart := time.Now()
	appCtx := ctx.Value(constant.AppContextKey).(*appContext.AppContext)

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		println("error meta data")
	}

	appCtx.Log().LogRequestGrpc(appCtx.ToContext(), method, "GRPC", &request, md)
	err := invoker(ctx, method, request, response, cc, opts...)

	if err != nil {
		appCtx.Log().LogResponseGrpc(appCtx.ToContext(), timeStart, method, "GRPC", err.Error())
		return err
	}
	appCtx.Log().LogResponseGrpc(appCtx.ToContext(), timeStart, method, "GRPC", &response)
	return err
}

//=========v2

//package grpc
//
//import (
//"appContext"
//"time"
//
//"github.com/google/uuid"
//"google.golang.org/grpc"
//"google.golang.org/grpc/metadata"
//)
//
//type Options struct {
//	Address string        `json:"address"`
//	Timeout time.Duration `json:"timeout"`
//}
//
//type RpcConnection struct {
//	options    Options
//	Connection *grpc.ClientConn
//}
//
//func (rpc *RpcConnection) CreateContext(parent appContext.Context, threadID uuid.UUID) (ctx appContext.Context, cancel appContext.CancelFunc) {
//	ctx, cancel = appContext.WithTimeout(parent, rpc.options.Timeout)
//	md := metadata.New(map[string]string{"Request-Id": threadID.String()})
//	ctx = metadata.NewOutgoingContext(ctx, md)
//	return
//}
//
//func New(options Options) (rpc *RpcConnection, err error) {
//	connection, err := grpc.Dial(options.Address, grpc.WithInsecure(), grpc.WithUnaryInterceptor(clientInterceptor))
//	if err != nil {
//		return nil, err
//	}
//
//	rpc = &RpcConnection{
//		Connection: connection,
//		options:    options,
//	}
//	return
//}
//
//func clientInterceptor(ctx appContext.Context, method string, request interface{}, response interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
//	timeStart := time.Now()
//	threadID := uuid.New()
//	md := metadata.Pairs("Request-Id", threadID.String())
//	ctx = metadata.NewOutgoingContext(ctx, md)
//	err := invoker(ctx, method, request, response, cc, opts...)
//
//	if err != nil {
//		return err
//	}
//	return err
//}
