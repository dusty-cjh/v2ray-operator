/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	vpnv1alpha1 "github.com/dusty-cjh/v2ray-operator/api/v1alpha1"
	"github.com/dusty-cjh/v2ray-operator/internal/managers/v2fly"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Definitions to manage status conditions
const (
	// typeAvailableV2ray represents the status of the Deployment reconciliation
	typeAvailableV2rayClient = "Available"
	// typeDegradedV2ray represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedV2rayClient = "Degraded"
	// typeInvalidParamsV2rayClient
	typeInvalidParamsV2rayClient = "InvalidParams"
	v2rayClientFinalizer         = "vpn.hdcjh.xyz/finalizer"

	v2rayEnableLabel = "v2ray.hdcjh.xyz/enable"
	v2rayRegionLabel = "v2ray.hdcjh.xyz/region"
	v2rayNamespace   = "vpn"
	v2raySvcPortName = "grpc"
)

// V2rayClientReconciler reconciles a V2rayClient object
type V2rayClientReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=vpn.hdcjh.xyz,resources=v2rayclients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vpn.hdcjh.xyz,resources=v2rayclients/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vpn.hdcjh.xyz,resources=v2rayclients/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the V2rayClient object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *V2rayClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var log = log.FromContext(ctx)

	// check whether the v2ray client resource exists
	v2ray := &vpnv1alpha1.V2rayClient{}
	err := r.Get(ctx, req.NamespacedName, v2ray)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("v2ray client resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get v2ray client")
		return ctrl.Result{}, err
	}

	// set status to unknown if the status is empty
	if v2ray.Status.Conditions == nil || len(v2ray.Status.Conditions) == 0 {
		meta.SetStatusCondition(&v2ray.Status.Conditions, metav1.Condition{Type: typeAvailableV2ray, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, v2ray); err != nil {
			log.Error(err, "Failed to update V2ray status")
			return ctrl.Result{}, err
		}
	}

	// validate the v2ray client config
	v4, err := v2ray.Spec.V2ray.ToV2rayV4Config()
	if err != nil {
		log.Error(err, "Failed to convert v2ray config to v4")

		//	set v2ray client status to error
		meta.SetStatusCondition(&v2ray.Status.Conditions, metav1.Condition{Type: typeInvalidParamsV2rayClient, Status: metav1.ConditionTrue, Reason: "FailedToConvertV2rayConfig", Message: err.Error()})

		return ctrl.Result{}, nil
	}
	log.Info("v2ray config", "v4", v4)

	//	elect nodes to deploy v2ray client config

	//	get service by label
	v2raySvcList := corev1.ServiceList{}
	err = r.List(ctx, &v2raySvcList, client.InNamespace(v2rayNamespace), client.MatchingLabels{
		v2rayEnableLabel: "true",
	})
	if err != nil {
		log.Error(err, "Failed to get v2ray svc list")
		return ctrl.Result{}, err
	}

	//	elect services by affinity
	svcList, err := r.ElectServicesFromAffinity(ctx, v2ray.Spec.Affinity)
	//	add v2ray config for each of these svc using grpc request
	v2flyCfgList, err := r.SvcListToV2flyConfig(ctx, svcList)
	for i, v2flyCfg := range v2flyCfgList {
		svc := svcList[i]
		err = r.applyV2rayClientConfig(ctx, &svc, v2ray.Spec.V2ray, &v2flyCfg)
		if err != nil {
			log.Error(err, "Failed to apply v2ray client config")
			return ctrl.Result{}, err
		}
	}

	//	official --------------------------------------------------------------------------------------------------

	// check the V2ray instance
	v2rayCli := &vpnv1alpha1.V2rayClient{}
	if err := r.Get(ctx, req.NamespacedName, v2rayCli); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("v2ray cli resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get v2ray")
		return ctrl.Result{}, err
	}

	//	Set status = Unknow for new object
	// Let's just set the status as Unknown when no status are available
	if v2rayCli.Status.Conditions == nil || len(v2rayCli.Status.Conditions) == 0 {
		meta.SetStatusCondition(&v2rayCli.Status.Conditions, metav1.Condition{Type: typeAvailableV2rayClient, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, v2rayCli); err != nil {
			log.Error(err, "Failed to update V2ray cli status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the v2ray Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, v2rayCli); err != nil {
			log.Error(err, "Failed to re-fetch v2ray cli")
			return ctrl.Result{}, err
		}
	}

	//	Destruct function:
	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(v2rayCli, v2rayClientFinalizer) {
		log.Info("Adding Finalizer for V2ray Cli")
		if ok := controllerutil.AddFinalizer(v2rayCli, v2rayClientFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err = r.Update(ctx, v2rayCli); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	//	Destruct:
	// Check if the V2ray instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isV2rayMarkedToBeDeleted := v2rayCli.GetDeletionTimestamp() != nil
	if isV2rayMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(v2rayCli, v2rayClientFinalizer) {
			log.Info("Performing Finalizer Operations for V2ray Cli before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&v2rayCli.Status.Conditions, metav1.Condition{Type: typeDegradedV2rayClient,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", v2rayCli.Name)})

			if err := r.Status().Update(ctx, v2rayCli); err != nil {
				log.Error(err, "Failed to update V2ray status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			if err := r.doFinalizerOperationsForV2rayCli(v2rayCli); err != nil {
				log.Error(err, "Failed to perform finalizer operations for V2ray Cli")
				return ctrl.Result{}, err
			}

			// TODO(user): If you add operations to the doFinalizerOperationsForV2ray method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the v2ray Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, v2rayCli); err != nil {
				log.Error(err, "Failed to re-fetch v2ray cli")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&v2rayCli.Status.Conditions, metav1.Condition{Type: typeDegradedV2rayClient,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", v2rayCli.Name)})

			if err := r.Status().Update(ctx, v2rayCli); err != nil {
				log.Error(err, "Failed to update V2ray status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for V2ray after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(v2rayCli, v2rayClientFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for V2ray")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, v2rayCli); err != nil {
				log.Error(err, "Failed to remove finalizer for V2ray Cli")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	//	release v2ray client status
	meta.SetStatusCondition(&v2rayCli.Status.Conditions, metav1.Condition{Type: typeAvailableV2rayClient,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) created successfully", v2rayCli.Name)})

	if err := r.Status().Update(ctx, v2rayCli); err != nil {
		log.Error(err, "Failed to update V2ray cli status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *V2rayClientReconciler) ElectServicesFromAffinity(ctx context.Context, affinity *vpnv1alpha1.V2rayNodeAffinity) ([]corev1.Service, error) {
	var log = log.FromContext(ctx)
	var ret = make([]corev1.Service, 0, len(affinity.Regions))

	//	get service by label
	v2raySvcList := corev1.ServiceList{}
	if err := r.List(ctx, &v2raySvcList, client.InNamespace(v2rayNamespace), client.MatchingLabels{
		v2rayEnableLabel: "true",
	}); err != nil {
		log.Error(err, "ElectServicesFromAffinity: failed to get v2ray svc list")
		return nil, err
	}

	//	convert regions affinity to map[region]num
	regionAffinityMap := make(map[string]int)
	for _, region := range affinity.Regions {
		regionAffinityMap[region.Region] = region.Nums
	}

	for _, svc := range v2raySvcList.Items {
		svcRegion, ok := svc.Labels[v2rayRegionLabel]
		if !ok {
			log.Info("ElectServicesFromAffinity: v2ray svc has no region label", "svc", svc.Name)
			continue
		}
		// check whether need this region
		num, ok := regionAffinityMap[svcRegion]
		if !ok || num == 0 {
			continue
		}
		// add this svc to result
		ret = append(ret, svc)
		regionAffinityMap[svcRegion] = num - 1
	}

	//	check regions not satisfied
	for k, v := range regionAffinityMap {
		if v == 0 {
			continue
		}
		log.Info("ElectServicesFromAffinity: region not satisfied", "region", k, "num", v)
	}

	return ret, nil
}

func (r *V2rayClientReconciler) SvcListToV2flyConfig(ctx context.Context, svcList []corev1.Service) ([]v2fly.V2rayConfig, error) {
	var log = log.FromContext(ctx)
	var ret = make([]v2fly.V2rayConfig, 0, len(svcList))

	//	get svc cluster ip
	for _, svc := range svcList {
		if svc.Spec.ClusterIP == "" {
			log.Info("SvcListToV2flyConfig: svc has no cluster ip", "svc", svc.Name)
			continue
		}

		//	get the port named grpc
		var grpcPort *corev1.ServicePort
		for _, port := range svc.Spec.Ports {
			if port.Name == v2raySvcPortName {
				grpcPort = &port
				break
			}
		}
		if grpcPort == nil {
			log.Info("SvcListToV2flyConfig: svc has no grpc port", "svc", svc.Name)
			continue
		}

		ret = append(ret, v2fly.V2rayConfig{
			Host: svc.Spec.ClusterIP,
			Port: int(grpcPort.Port),
		})
	}

	return ret, nil
}

func (r *V2rayClientReconciler) applyV2rayClientConfig(ctx context.Context, svc *corev1.Service, v2ray *v2fly.V2rayInstanceConfig, v2rayCliCfg *v2fly.V2rayConfig) error {
	var log = log.FromContext(ctx)

	//	connect to v2ray grpc server
	cli := v2fly.NewV2rayGrpcApi(v2rayCliCfg)

	//	inbound
	inbounds := v2ray.Inbounds
	if len(inbounds) != 0 {
		if err := cli.AddInbound(ctx, v2ray); err != nil {
			log.Error(err, "applyV2rayClientConfig: failed to add inbound")
			return err
		}
	}

	//	outbound
	outbounds := v2ray.Outbounds
	if len(outbounds) != 0 {
		if err := cli.AddOutbound(ctx, v2ray); err != nil {
			log.Error(err, "applyV2rayClientConfig: failed to add outbound")
			return err
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *V2rayClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vpnv1alpha1.V2rayClient{}).
		Complete(r)
}

func (r *V2rayClientReconciler) doFinalizerOperationsForV2rayCli(v2rayCli *vpnv1alpha1.V2rayClient) error {
	// Let's add here the operations that should be done before delete the custom resource
	// such as delete the Deployment, Service, etc.
	return nil
}
