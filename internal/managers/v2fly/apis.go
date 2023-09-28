package v2fly

import (
	"context"
	"fmt"
	"github.com/gogo/protobuf/proto"
	handlerService "github.com/v2fly/v2ray-core/v5/app/proxyman/command"
	statsService "github.com/v2fly/v2ray-core/v5/app/stats/command"
	"github.com/v2fly/v2ray-core/v5/common/serial"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	log2 "sigs.k8s.io/controller-runtime/pkg/log"
)

type AlterInboundInterface interface {
	proto.Message
	handlerService.InboundOperation
}

type AlterOutboundInterface interface {
	proto.Message
	handlerService.OutboundOperation
}

type V2rayGrpcApiInterface interface {
	GetSysStats(ctx context.Context) (*statsService.SysStatsResponse, error)
	QueryStats(ctx context.Context, r *statsService.QueryStatsRequest) (*statsService.QueryStatsResponse, error)
	AddInbound(ctx context.Context, cfg *V2rayInstanceConfig) error
	RemoveInbound(ctx context.Context, cfg *V2rayInstanceConfig) error
	AlterInbound(ctx context.Context, tag string, operation AlterInboundInterface) error
	AddOutbound(ctx context.Context, cfg *V2rayInstanceConfig) error
	RemoveOutbound(ctx context.Context, cfg *V2rayInstanceConfig) error
	AlterOutbound(ctx context.Context, tag string, operation AlterOutboundInterface) error
}

type V2rayGrpcApi struct {
	Conn   *grpc.ClientConn
	Config *V2rayConfig
}

func NewV2rayGrpcApi(cfg *V2rayConfig) *V2rayGrpcApi {
	//	create grpc connection
	apiServerAddrPtr := cfg.ServerAddr()
	conn, err := grpc.DialContext(
		context.Background(),
		apiServerAddrPtr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		panic(fmt.Errorf("failed to dial v2ray api server: %w", err))
	}

	//	create api client
	return &V2rayGrpcApi{
		Conn:   conn,
		Config: cfg,
	}
}

func (this *V2rayGrpcApi) QueryStats(
	ctx context.Context,
	r *statsService.QueryStatsRequest,
) (*statsService.QueryStatsResponse, error) {
	client := statsService.NewStatsServiceClient(this.Conn)
	resp, err := client.QueryStats(ctx, r)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (this *V2rayGrpcApi) GetSysStats(ctx context.Context) (*statsService.SysStatsResponse, error) {
	client := statsService.NewStatsServiceClient(this.Conn)
	r := &statsService.SysStatsRequest{}
	resp, err := client.GetSysStats(ctx, r)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (this *V2rayGrpcApi) AddInbound(ctx context.Context, cfg *V2rayInstanceConfig) error {
	cli := handlerService.NewHandlerServiceClient(this.Conn)
	log := log2.FromContext(ctx)

	//	k8s config to v2ray config
	c, err := cfg.ToV2rayV4Config()
	if err != nil {
		log.Error(err, "V2rayGrpcApi.AddInbound failed to convert k8s config to v2ray config")
		return fmt.Errorf("V2rayGrpcApi.ToV2rayV4Config error: %w", err)
	}
	if len(c.InboundConfigs) == 0 {
		log.Error(err, "V2rayGrpcApi.AddInbound no valid inbound found")
		return fmt.Errorf("no valid inbound found")
	}

	//	add inbound
	for idx, in := range c.InboundConfigs {
		log.Info(fmt.Sprintf("[%d/%d] V2rayGrpcApi.AddInbound adding: %s", idx+1, len(c.InboundConfigs), in.Tag))
		i, err := in.Build()
		if err != nil {
			log.Error(err, "V2rayGrpcApi.AddInbound failed to build i conf: %v")
			return fmt.Errorf("V2rayGrpcApi.AddInbound failed to build i conf: %w", err)
		}
		r := &handlerService.AddInboundRequest{
			Inbound: i,
		}
		_, err = cli.AddInbound(ctx, r)
		if err != nil {
			log.Error(err, "V2rayGrpcApi.AddInbound failed to add inbound")
			return fmt.Errorf("V2rayGrpcApi.AddInbound failed to add inbound: %w", err)
		}
	}

	return nil
}

func (this *V2rayGrpcApi) RemoveInbound(ctx context.Context, cfg *V2rayInstanceConfig) error {
	cli := handlerService.NewHandlerServiceClient(this.Conn)
	log := log2.FromContext(ctx)

	//	k8s config to v2ray config
	c, err := cfg.ToV2rayV4Config()
	if err != nil {
		log.Error(err, "V2rayGrpcApi.RemoveInbound failed to convert k8s config to v2ray config")
		return fmt.Errorf("V2rayGrpcApi.RemoveInbound to v4cfg error: %w", err)
	}
	if len(c.InboundConfigs) == 0 {
		log.Error(err, "V2rayGrpcApi.RemoveInbound no valid inbound found")
		return fmt.Errorf("V2rayGrpcApi.RemoveInbound no valid inbound found")
	}

	//	get tags
	var tags []string
	tags = make([]string, 0, len(c.InboundConfigs))
	for _, c := range c.InboundConfigs {
		if c.Tag == "" {
			log.Error(err, "V2rayGrpcApi.RemoveInbound empty inbound tag detected")
			return fmt.Errorf("V2rayGrpcApi.RemoveInbound empty inbound tag detected")
		}
		tags = append(tags, c.Tag)
	}

	//	rmv inbound
	for i, tag := range tags {
		log.Info(fmt.Sprintf("[%d/%d] V2rayGrpcApi.RemoveInbound removing: %s", i+1, len(tags), tag))
		r := &handlerService.RemoveInboundRequest{
			Tag: tag,
		}
		_, err := cli.RemoveInbound(ctx, r)
		if err != nil {
			log.Error(err, "V2rayGrpcApi.RemoveInbound failed to remove inbound")
			return fmt.Errorf("V2rayGrpcApi.RemoveInbound failed to remove inbound: %w", err)
		}
	}

	return nil
}

func (this *V2rayGrpcApi) AlterInbound(ctx context.Context, tag string, operation AlterInboundInterface) error {
	cli := handlerService.NewHandlerServiceClient(this.Conn)
	log := log2.FromContext(ctx)

	//	parse operation
	op := serial.ToTypedMessage(operation)

	//	make grpc request
	r := &handlerService.AlterInboundRequest{
		Tag:       tag,
		Operation: op,
	}
	_, err := cli.AlterInbound(ctx, r)
	if err != nil {
		log.Error(err, "V2rayGrpcApi.AlterInbound failed to alter inbound")
		return fmt.Errorf("V2rayGrpcApi.AlterInbound failed to alter inbound: %w", err)
	}

	return nil
}

func (this *V2rayGrpcApi) AddOutbound(ctx context.Context, cfg *V2rayInstanceConfig) error {
	cli := handlerService.NewHandlerServiceClient(this.Conn)
	log := log2.FromContext(ctx)

	//	k8s config to v2ray config
	c, err := cfg.ToV2rayV4Config()
	if err != nil {
		log.Error(err, "V2rayGrpcApi.AddOutbound failed to convert k8s config to v2ray config")
		return fmt.Errorf("V2rayGrpcApi.ToV2rayV4Config error: %w", err)
	}
	if len(c.OutboundConfigs) == 0 {
		log.Error(err, "V2rayGrpcApi.AddOutbound no valid inbound found")
		return fmt.Errorf("no valid inbound found")
	}

	//	add inbound
	for idx, out := range c.OutboundConfigs {
		log.Info(fmt.Sprintf("[%d/%d] V2rayGrpcApi.AddOutbound adding: %s", idx+1, len(c.OutboundConfigs), out.Tag))
		o, err := out.Build()
		if err != nil {
			log.Error(err, "V2rayGrpcApi.AddOutbound failed to build i conf: %v")
			return fmt.Errorf("V2rayGrpcApi.AddOutbound failed to build i conf: %w", err)
		}
		r := &handlerService.AddOutboundRequest{
			Outbound: o,
		}
		_, err = cli.AddOutbound(ctx, r)
		if err != nil {
			log.Error(err, "V2rayGrpcApi.AddOutbound failed to add inbound")
			return fmt.Errorf("V2rayGrpcApi.AddOutbound failed to add inbound: %w", err)
		}
	}

	return nil
}

func (this *V2rayGrpcApi) RemoveOutbound(ctx context.Context, cfg *V2rayInstanceConfig) error {
	cli := handlerService.NewHandlerServiceClient(this.Conn)
	log := log2.FromContext(ctx)

	//	k8s config to v2ray config
	c, err := cfg.ToV2rayV4Config()
	if err != nil {
		log.Error(err, "V2rayGrpcApi.RemoveOutbound failed to convert k8s config to v2ray config")
		return fmt.Errorf("V2rayGrpcApi.RemoveOutbound to v4cfg error: %w", err)
	}
	if len(c.OutboundConfigs) == 0 {
		log.Error(err, "V2rayGrpcApi.RemoveOutbound no valid inbound found")
		return fmt.Errorf("V2rayGrpcApi.RemoveOutbound no valid inbound found")
	}

	//	get tags
	var tags []string
	tags = make([]string, 0, len(c.OutboundConfigs))
	for _, c := range c.OutboundConfigs {
		if c.Tag == "" {
			log.Error(err, "V2rayGrpcApi.RemoveOutbound empty inbound tag detected")
			return fmt.Errorf("V2rayGrpcApi.RemoveOutbound empty inbound tag detected")
		}
		tags = append(tags, c.Tag)
	}

	//	rmv inbound
	for i, tag := range tags {
		log.Info(fmt.Sprintf("[%d/%d] V2rayGrpcApi.RemoveOutbound removing: %s", i+1, len(tags), tag))
		r := &handlerService.RemoveOutboundRequest{
			Tag: tag,
		}
		_, err := cli.RemoveOutbound(ctx, r)
		if err != nil {
			log.Error(err, "V2rayGrpcApi.RemoveOutbound failed to remove inbound")
			return fmt.Errorf("V2rayGrpcApi.RemoveOutbound failed to remove inbound: %w", err)
		}
	}

	return nil
}

func (this *V2rayGrpcApi) AlterOutbound(ctx context.Context, tag string, operation AlterOutboundInterface) error {
	cli := handlerService.NewHandlerServiceClient(this.Conn)
	log := log2.FromContext(ctx)

	//	parse operation
	op := serial.ToTypedMessage(operation)

	//	make grpc request
	r := &handlerService.AlterOutboundRequest{
		Tag:       tag,
		Operation: op,
	}
	_, err := cli.AlterOutbound(ctx, r)
	if err != nil {
		log.Error(err, "V2rayGrpcApi.AlterOutbound failed to alter inbound")
		return fmt.Errorf("V2rayGrpcApi.AlterOutbound failed to alter inbound: %w", err)
	}

	return nil
}
