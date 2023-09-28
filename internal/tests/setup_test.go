package tests_test

import (
	"context"
	"fmt"
	"github.com/dusty-cjh/v2ray-operator/internal/managers"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

var (
	k8sConfig    *rest.Config
	k8sClientset *kubernetes.Clientset
	k8sClient    client.Client
)

func TestMain(m *testing.M) {
	//	get config
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		panic(fmt.Errorf("error building kubeconfig: %s", err.Error()))
	}
	k8sConfig = config

	//	get client set
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(fmt.Errorf("error building kubernetes clientset: %s", err.Error()))
	}
	k8sClientset = clientset

	//	get client
	c, err := client.New(config, client.Options{})
	if err != nil {
		panic(fmt.Errorf("error building kubernetes client: %s", err.Error()))
	}
	k8sClient = c

	m.Run()
}

func TestGetPodsByLabels(t *testing.T) {
	ctx := context.Background()
	pods, err := k8sClientset.CoreV1().Pods("vpn").List(ctx, metav1.ListOptions{
		LabelSelector: "app=v2ray",
	})
	assert.Nil(t, err)

	podList := &corev1.PodList{}
	err = k8sClient.List(context.TODO(), podList, client.InNamespace("vpn"), client.MatchingLabels{"app": "v2ray"})
	assert.Nil(t, err)

	assert.Equal(t, len(podList.Items), len(pods.Items))
	for i, pod := range podList.Items {
		pod2 := pods.Items[i]
		assert.Equal(t, pod.Name, pod2.Name)
		assert.Equal(t, pod.Namespace, pod2.Namespace)
		assert.Equal(t, pod.Labels, pod2.Labels)
		assert.Equal(t, pod.Spec, pod2.Spec)
	}
}

func TestGetNodes(t *testing.T) {
	ctx := context.Background()
	nodes, err := k8sClientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	assert.Nil(t, err)

	nodeList := &corev1.NodeList{}
	err = k8sClient.List(context.TODO(), nodeList, client.MatchingLabels{})
	assert.Nil(t, err)

	assert.Equal(t, len(nodeList.Items), len(nodes.Items))
	for i, node := range nodeList.Items {
		node2 := nodes.Items[i]
		// validate node host and ip info
		assert.Equal(t, node.Name, node2.Name)
		assert.Equal(t, node.UID, node2.UID)
	}
}

func TestGetServiceByName(t *testing.T) {
	ctx := context.Background()
	svc, err := k8sClientset.CoreV1().Services("vpn").Get(ctx, "v2ray", metav1.GetOptions{})
	assert.Nil(t, err)

	svc2 := &corev1.Service{}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: "vpn",
		Name:      "v2ray",
	}, svc2)
	assert.Nil(t, err)

	assert.Equal(t, svc.Name, svc2.Name)
	assert.Equal(t, svc.Namespace, svc2.Namespace)
	assert.Equal(t, svc.Labels, svc2.Labels)
	assert.Equal(t, svc.Spec, svc2.Spec)
}

func TestGetV2rayConfigFromService(t *testing.T) {
	ctx := context.Background()
	cli := managers.NewK8sManager(k8sClient)
	cfg, err := cli.GetV2rayConfigFromService(ctx, "v2ray", "vpn")
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	t.Logf("cfg: %+v", cfg)
}

func TestV2rayGrpcApi(t *testing.T) {
	//ctx := context.Background()

}
