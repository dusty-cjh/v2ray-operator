package managers

import (
	"context"
	"errors"
	"github.com/dusty-cjh/v2ray-operator/internal/managers/v2fly"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

const (
	V2rayGrpcPortName = "grpc"
)

type K8SManager struct {
	Cli client.Client
}

func NewK8sManager(cli client.Client) *K8SManager {
	return &K8SManager{
		Cli: cli,
	}
}

func (this *K8SManager) GetV2rayConfigFromService(
	ctx context.Context, name string, namespace string) (*v2fly.V2rayConfig, error) {
	svc := &corev1.Service{}
	err := this.Cli.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, svc)
	if err != nil {
		return nil, err
	}
	host := svc.Spec.ClusterIP
	port := 0
	for _, pc := range svc.Spec.Ports {
		if pc.Name == V2rayGrpcPortName {
			port = int(pc.Port)
			break
		}
	}
	if port == 0 {
		return nil, errors.New("no grpc port found")
	}

	return &v2fly.V2rayConfig{
		Host: host,
		Port: port,
	}, nil
}

func (this *K8SManager) GetV2rayClientFromService(
	ctx context.Context, name string, namespace string) (*v2fly.V2rayGrpcApi, error) {
	cfg, err := this.GetV2rayConfigFromService(ctx, name, namespace)
	if err != nil {
		return nil, err
	}
	return v2fly.NewV2rayGrpcApi(cfg), nil
}

func ENV() string {
	//	get env from env var
	env := os.Getenv("ENV")
	if env == "" {
		env = "test"
	}
	return strings.ToLower(env)
}
