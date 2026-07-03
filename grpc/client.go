package grpc

import (
	"context"
	"time"

	"github.com/ewinjuman/go-lib/v2/appContext"
	"github.com/ewinjuman/go-lib/v2/constant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Options struct {
	// Address is the target gRPC server address (host:port).
	Address string `json:"address"`
	// Timeout is the RPC context deadline, used as-is in CreateContext
	// (e.g. Timeout: 5*time.Second) — not a bare count of seconds.
	Timeout time.Duration `json:"timeout"`
}

type RpcConnection struct {
	options    Options
	Connection *grpc.ClientConn
}

func (rpc *RpcConnection) CreateContext(parent context.Context, appCtx *appContext.AppContext) (ctx context.Context, cancel context.CancelFunc) {
	ctx, cancel = context.WithTimeout(parent, rpc.options.Timeout)
	ctx = context.WithValue(ctx, constant.AppContextKey, appCtx)
	md := metadata.New(map[string]string{"Request-Id": appCtx.GetRequestID()})
	ctx = metadata.NewOutgoingContext(ctx, md)
	return
}

func New(options Options) (rpc *RpcConnection, err error) {
	connection, err := grpc.NewClient(
		options.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(clientInterceptor),
	)
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

	appCtx, ok := ctx.Value(constant.AppContextKey).(*appContext.AppContext)
	if !ok || appCtx == nil {
		return invoker(ctx, method, request, response, cc, opts...)
	}

	md, _ := metadata.FromOutgoingContext(ctx)

	appCtx.Log().LogRequestGrpc(method, "GRPC", &request, md)
	err := invoker(ctx, method, request, response, cc, opts...)

	if err != nil {
		appCtx.Log().LogResponseGrpc(timeStart, method, "GRPC", err.Error())
		return err
	}
	appCtx.Log().LogResponseGrpc(timeStart, method, "GRPC", &response)
	return err
}
