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
	"errors"
	"fmt"
	"github.com/dusty-cjh/v2ray-operator/internal/constant"
	"github.com/dusty-cjh/v2ray-operator/internal/managers"
	"github.com/dusty-cjh/v2ray-operator/internal/managers/v2fly"
	"github.com/google/uuid"
	"github.com/v2fly/v2ray-core/v5/app/proxyman/command"
	"github.com/v2fly/v2ray-core/v5/common/protocol"
	"github.com/v2fly/v2ray-core/v5/common/serial"
	"github.com/v2fly/v2ray-core/v5/proxy/vless"
	"github.com/v2fly/v2ray-core/v5/proxy/vmess"
	"google.golang.org/protobuf/types/known/anypb"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	vpnv1alpha1 "github.com/dusty-cjh/v2ray-operator/api/v1alpha1"
)

const v2rayuserFinalizer = "vpn.hdcjh.xyz/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailableV2rayUser represents the status of the Deployment reconciliation
	typeAvailableV2rayUser = "Available"
	// typeDegradedV2rayUser represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedV2rayUser = "Degraded"
	//
	v2rayDefaultVlessTag = "ws+vless"
	v2rayDefaultVmessTag = "ws+vmess"
)

// V2rayUserReconciler reconciles a V2rayUser object
type V2rayUserReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// The following markers are used to generate the rules permissions (RBAC) on config/rbac using controller-gen
// when the command <make manifests> is executed.
// To know more about markers see: https://book.kubebuilder.io/reference/markers.html

//+kubebuilder:rbac:groups=vpn.hdcjh.xyz,resources=v2rayusers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vpn.hdcjh.xyz,resources=v2rayusers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vpn.hdcjh.xyz,resources=v2rayusers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch;list
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// It is essential for the controller's reconciliation loop to be idempotent. By following the Operator
// pattern you will create Controllers which provide a reconcile function
// responsible for synchronizing resources until the desired state is reached on the cluster.
// Breaking this recommendation goes against the design principles of controller-runtime.
// and may lead to unforeseen consequences such as resources becoming stuck and requiring manual intervention.
// For further info:
// - About Operator Pattern: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
// - About Controllers: https://kubernetes.io/docs/concepts/architecture/controller/
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *V2rayUserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the V2rayUser instance
	// The purpose is check if the Custom Resource for the Kind V2rayUser
	// is applied on the cluster if not we return nil to stop the reconciliation
	v2rayuser := &vpnv1alpha1.V2rayUser{}
	err := r.Get(ctx, req.NamespacedName, v2rayuser)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("v2rayuser resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get v2rayuser")
		return ctrl.Result{}, err
	}

	// init Status to Unknown
	if v2rayuser.Status.Conditions == nil || len(v2rayuser.Status.Conditions) == 0 {
		meta.SetStatusCondition(&v2rayuser.Status.Conditions, metav1.Condition{Type: typeAvailableV2rayUser, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, v2rayuser); err != nil {
			log.Error(err, "Failed to update V2rayUser status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the v2rayuser Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err = r.Get(ctx, req.NamespacedName, v2rayuser); err != nil {
			log.Error(err, "Failed to re-fetch v2rayuser")
			return ctrl.Result{}, err
		}
	}

	//	generate uuid if not provided
	if v2rayuser.Spec.User.Id == "" {
		v2rayuser.Spec.User.Id = uuid.New().String()
	}
	//	update result
	if err = r.Update(ctx, v2rayuser); err != nil {
		log.Error(err, "Failed to update v2rayuser")
		return ctrl.Result{}, err
	}
	if err = r.Get(ctx, req.NamespacedName, v2rayuser); err != nil {
		log.Error(err, "Failed to re-fetch v2rayuser")
		return ctrl.Result{}, err
	}
	log.Info(fmt.Sprintf("V2rayUser %s auto generated uuid: %s", v2rayuser.Spec.User.Email, v2rayuser.Spec.User.Id))

	//	Elect nodes that satisfy the v2ray user affinity
	svcList, err := r.electServicesFromAffinity(ctx, v2rayuser.Spec.NodeList)
	if err != nil {
		log.Error(err, "Failed to elect nodes from affinity")
		return ctrl.Result{}, err
	}
	if len(svcList) == 0 {
		log.Info("No node elected from affinity")

		//	Update v2ray user status to NoNodeElected
		meta.SetStatusCondition(&v2rayuser.Status.Conditions, metav1.Condition{Type: typeAvailableV2rayUser,
			Status: metav1.ConditionFalse, Reason: "NoNodeElected",
			Message: fmt.Sprintf("No node elected from affinity")})
		if err := r.Status().Update(ctx, v2rayuser); err != nil {
			log.Error(err, "Failed to update V2rayUser status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
	log.Info(fmt.Sprintf("Elected %d svc from affinity for %s", len(svcList), v2rayuser.Spec.User.Id))

	//	add user to v2ray svc list
	//	TODO: validate and format the v2ray user info
	err = r.addUserToV2raySvcList(ctx, v2rayuser, svcList)
	if err != nil {
		log.Error(err, "Failed to add user to v2ray svc list")

		//	Update v2ray user status to V2raySvcAddFailed
		meta.SetStatusCondition(&v2rayuser.Status.Conditions, metav1.Condition{Type: typeAvailableV2rayUser,
			Status: metav1.ConditionFalse, Reason: "V2raySvcAddFailed",
			Message: fmt.Sprintf("Failed to add user to v2ray svc list: %v", err)})
		if err := r.Status().Update(ctx, v2rayuser); err != nil {
			log.Error(err, "Failed to update V2rayUser status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	} else {
		r.Recorder.Event(v2rayuser, "Warning", "V2raySvcAdded",
			fmt.Sprintf("V2ray Svc Grpc call success, %s v2rayuser is being added to the server, namespace %s",
				v2rayuser.Name,
				v2rayuser.Namespace))
	}
	log.Info("Add user to v2ray svc list success")

	// add finalizer
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(v2rayuser, v2rayuserFinalizer) {
		log.Info("Adding Finalizer for V2rayUser")
		if ok := controllerutil.AddFinalizer(v2rayuser, v2rayuserFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err = r.Update(ctx, v2rayuser); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}
	log.Info("Finalizer added to the V2rayUser")

	// Check if the V2rayUser instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isV2rayUserMarkedToBeDeleted := v2rayuser.GetDeletionTimestamp() != nil
	if isV2rayUserMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(v2rayuser, v2rayuserFinalizer) {
			log.Info("Performing Finalizer for V2rayUser before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&v2rayuser.Status.Conditions, metav1.Condition{Type: typeDegradedV2rayUser,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", v2rayuser.Name)})

			if err := r.Status().Update(ctx, v2rayuser); err != nil {
				log.Error(err, "Failed to update V2rayUser status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			if err := r.doFinalizerOperationsForV2rayUser(ctx, v2rayuser); err != nil {
				log.Error(err, "Failed to perform finalizer operations for V2rayUser")
				return ctrl.Result{}, err
			}

			// TODO(user): If you add operations to the doFinalizerOperationsForV2rayUser method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the v2rayuser Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, v2rayuser); err != nil {
				log.Error(err, "Failed to re-fetch v2rayuser")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&v2rayuser.Status.Conditions, metav1.Condition{Type: typeDegradedV2rayUser,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", v2rayuser.Name)})

			if err := r.Status().Update(ctx, v2rayuser); err != nil {
				log.Error(err, "Failed to update V2rayUser status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for V2rayUser after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(v2rayuser, v2rayuserFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for V2rayUser")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, v2rayuser); err != nil {
				log.Error(err, "Failed to remove finalizer for V2rayUser")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// set available status to true
	meta.SetStatusCondition(&v2rayuser.Status.Conditions, metav1.Condition{Type: typeAvailableV2rayUser,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) with %d replicas created successfully", v2rayuser.Name, len(v2rayuser.Spec.NodeList))})
	if err := r.Status().Update(ctx, v2rayuser); err != nil {
		log.Error(err, "Failed to update V2rayUser status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// finalizeV2rayUser will perform the required operations before delete the CR.
func (r *V2rayUserReconciler) doFinalizerOperationsForV2rayUser(ctx context.Context, cr *vpnv1alpha1.V2rayUser) (err error) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	var log = log.FromContext(ctx).WithValues("func", "doFinalizerOperationsForV2rayUser")
	var user = cr.Spec.User
	var tag = cr.Spec.InboundTag
	var account *anypb.Any
	log.Info("start delete v2ray user")
	switch {
	case strings.Contains(cr.Spec.InboundTag, "vless"):
		account = serial.ToTypedMessage(&vless.Account{
			Id: user.Id,
		})
	case strings.Contains(cr.Spec.InboundTag, "vmess"):
		account = serial.ToTypedMessage(&vmess.Account{
			Id:      user.Id,
			AlterId: 0,
		})
	default:
		log.Info("doFinalizerOperationsForV2rayUser: invalid inbound tag")
		return fmt.Errorf("doFinalizerOperationsForV2rayUser: unsupported inbound tag")
	}
	v2rayUser := &protocol.User{
		Level:   uint32(user.Level),
		Email:   user.Email,
		Account: account,
	}

	//
	successedSvcList := make([]*corev1.Service, 0, len(cr.Spec.NodeList))
	defer func() {
		rec := recover()
		if err == nil && rec == nil {
			return
		}

		//	rollback
		for _, svc := range successedSvcList {
			region := svc.Labels[v2rayRegionLabel]
			cli, err := getV2rayGrpcClientFromSvc(svc)
			if err != nil {
				log.Error(err, "doFinalizerOperationsForV2rayUser.rollback: failed to get v2ray grpc client", "region", region)
				continue
			}

			//	delete user
			if err := cli.AlterInbound(
				ctx, tag,
				&command.AddUserOperation{
					User: v2rayUser,
				}); err != nil {
				log.Error(err, "doFinalizerOperationsForV2rayUser.rollback: failed to delete user from v2ray client config", "region", region)
				continue
			}
		}

		//	continue panic
		if rec != nil {
			panic(rec)
		}
	}()

	//
	for _, nl := range cr.Spec.NodeList {
		for _, ni := range nl.Info {
			//	get svc by svc name
			svc := &corev1.Service{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: v2rayNamespace, Name: ni.SvcName}, svc); err != nil {
				log.Error(err, "doFinalizerOperationsForV2rayUser: Failed to get svc", "svc", ni.SvcName)
				return err
			}

			//	get v2fly client
			cli, err := getV2rayGrpcClientFromSvc(svc)
			if err != nil {
				log.Error(err, "doFinalizerOperationsForV2rayUser: Failed to get v2fly client", "svc", ni.SvcName)
				return err
			}

			//	remove user from v2ray
			if err := cli.AlterInbound(ctx, tag, &command.RemoveUserOperation{
				Email: user.Email,
			}); err != nil {
				log.Error(err, "doFinalizerOperationsForV2rayUser: Failed to remove user from v2ray", "svc", ni.SvcName)
				return err
			}

			//
			successedSvcList = append(successedSvcList, svc)
		}
	}

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("V2rayUser name=%s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))

	return nil
}

// labelsForV2rayUser returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForV2rayUser(name string) map[string]string {
	var imageTag string
	image, err := imageForV2rayUser()
	if err == nil {
		imageTag = strings.Split(image, ":")[1]
	}
	return map[string]string{"app.kubernetes.io/name": "V2rayUser",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "v2ray-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// imageForV2rayUser gets the Operand image which is managed by this controller
// from the V2RAYUSER_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForV2rayUser() (string, error) {
	var imageEnvVar = "V2RAYUSER_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return "", fmt.Errorf("Unable to find %s environment variable with the image", imageEnvVar)
	}
	return image, nil
}

// SetupWithManager sets up the controller with the Manager.
// Note that the Deployment will be also watched in order to ensure its
// desirable state on the cluster
func (r *V2rayUserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vpnv1alpha1.V2rayUser{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// define min func
func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func (r *V2rayUserReconciler) electServicesFromAffinity(ctx context.Context, nodeList map[string]*vpnv1alpha1.V2rayNodeList) ([]corev1.Service, error) {
	var log = log.FromContext(ctx)
	var ret = make([]corev1.Service, 0, len(nodeList))

	for region, affinity := range nodeList {
		if affinity.Size == 0 {
			affinity.Size = 1
		}

		//	global
		region = strings.ToLower(region)

		//	get service by label
		v2raySvcList := corev1.ServiceList{}
		if err := r.List(
			ctx, &v2raySvcList,
			client.InNamespace(v2rayNamespace),
			client.MatchingLabels{v2rayEnableLabel: "true"},
			client.MatchingLabels{v2rayRegionLabel: region}); err != nil {
			//	if not found
			if apierrors.IsNotFound(err) {
				log.Info("ElectServicesFromAffinity: v2ray svc not found", "region", region)
				continue
			}
			log.Error(err, "ElectServicesFromAffinity: failed to list v2ray svc", "region", region)
			return nil, err
		}
		if len(v2raySvcList.Items) == 0 {
			log.Info("ElectServicesFromAffinity: v2ray svc empty", "region", region)
			continue
		}

		//	add service to result
		size := min(len(v2raySvcList.Items), int(affinity.Size))
		//	TODO: random select service
		ret = append(ret, v2raySvcList.Items[:size]...)
	}

	return ret, nil
}

func getV2rayGrpcClientFromSvc(svc *corev1.Service) (*v2fly.V2rayGrpcApi, error) {
	//	get ip
	ip := svc.Spec.ClusterIP
	if managers.ENV() == constant.ENV_TEST {
		ip = "127.0.0.1"
	}

	//	get the svc  port named grpc
	port := int32(0)
	for _, p := range svc.Spec.Ports {
		if p.Name == v2raySvcPortName {
			port = p.Port
			break
		}
	}
	if port == 0 {
		return nil, errors.New("GetV2rayGrpcClientFromSvc: grpc port not found")
	}

	// get v2fly client
	v2flyConfig := &v2fly.V2rayConfig{
		Host: ip,
		Port: int(port),
	}
	cli := v2fly.NewV2rayGrpcApi(v2flyConfig)
	return cli, nil
}

func (r *V2rayUserReconciler) addUserToV2raySvcList(ctx context.Context, v2rayuser *vpnv1alpha1.V2rayUser, svcList []corev1.Service) (err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	var log = log.FromContext(ctx)
	var account *anypb.Any
	var nodeList = make(map[string]*vpnv1alpha1.V2rayNodeList)
	var v2rayInboundTag = v2rayuser.Spec.InboundTag
	var user = v2rayuser.Spec.User
	switch {
	case strings.Contains(v2rayInboundTag, "vless"):
		account = serial.ToTypedMessage(&vless.Account{
			Id: user.Id,
		})
	case strings.Contains(v2rayInboundTag, "vmess"):
		account = serial.ToTypedMessage(&vmess.Account{
			Id:      user.Id,
			AlterId: 0,
		})
	default:
		log.Info("AddUserToV2raySvcList: invalid inbound tag", "inboundTag", v2rayInboundTag)
		return fmt.Errorf("AddUserToV2raySvcList: unsupported inbound tag %s", v2rayInboundTag)
	}
	v2rayUser := &protocol.User{
		Level:   uint32(user.Level),
		Email:   user.Email,
		Account: account,
	}

	//	if return error, rollback
	successedSvcList := make([]corev1.Service, 0, len(svcList))
	defer func() {
		rec := recover()
		if err == nil && rec == nil {
			return
		}

		//	rollback
		for _, svc := range successedSvcList {
			region := svc.Labels[v2rayRegionLabel]
			cli, err := getV2rayGrpcClientFromSvc(&svc)
			if err != nil {
				log.Error(err, "AddUserToV2raySvcList.rollback: failed to get v2ray grpc client", "region", region)
				continue
			}

			//	delete user
			if err := cli.AlterInbound(
				ctx, v2rayInboundTag,
				&command.RemoveUserOperation{
					Email: user.Email,
				}); err != nil {
				log.Error(err, "AddUserToV2raySvcList.rollback: failed to delete user from v2ray client config", "region", region)
				continue
			}
		}

		//	continue panic
		if rec != nil {
			panic(rec)
		}
	}()

	for _, svc := range svcList {
		region := svc.Labels[v2rayRegionLabel]
		cli, err := getV2rayGrpcClientFromSvc(&svc)
		if err != nil {
			log.Error(err, "AddUserToV2raySvcList: failed to get v2ray grpc client", "region", region)
			return fmt.Errorf("AddUserToV2raySvcList: failed to get v2ray grpc client: %w", err)
		}

		// add user to v2fly client config
		if err = cli.AlterInbound(
			ctx, v2rayInboundTag,
			&command.AddUserOperation{
				User: v2rayUser,
			}); err != nil {
			log.Error(err, "AddUserToV2raySvcList: failed to add user to v2ray client config", "region", region)
			return fmt.Errorf("AddUserToV2raySvcList: failed to add user to v2ray client config: %w", err)
		}

		//	add svc related node to node list
		//	get svc related deployment
		dep := &appsv1.DeploymentList{}
		cond := svc.Spec.Selector
		if err := r.List(ctx, dep, client.InNamespace(svc.Namespace), client.MatchingLabels(cond)); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Failed to list deployment")
				return err
			}
			log.Error(err, "Failed to list deployment")
			return err
		}
		if len(dep.Items) == 0 {
			log.Error(nil, "No matched deployment found")
			return errors.New("no matched deployment found")
		}
		//	get deployment related pod
		pod := &corev1.PodList{}
		cond = dep.Items[0].Spec.Selector.MatchLabels
		if err := r.List(ctx, pod, client.InNamespace(dep.Items[0].Namespace), client.MatchingLabels(cond)); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "Failed to list pod")
				return err
			}
			log.Error(err, "Failed to list pod")
			return err
		}
		if len(pod.Items) == 0 {
			log.Error(nil, "Failed to get pod")
			return errors.New("failed to get pod")
		}
		p := pod.Items[0]
		nodeName := p.Spec.NodeName

		//	get node by node name
		node := &corev1.Node{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: p.Namespace, Name: nodeName}, node); err != nil {
			log.Error(err, "Failed to get node")
			return err
		}
		//	get node public address from metadata.annotations.flannel.alpha.coreos.com/public-ip
		pubIp, err := getNodePublicIp(node)
		if err != nil {
			log.Error(err, "Failed to get node public ip")
			return err
		}

		//	add svc related node to node list
		nl, ok := nodeList[region]
		if !ok {
			nl = &vpnv1alpha1.V2rayNodeList{}
			nodeList[region] = nl
		}
		nl.Info = append(nl.Info, vpnv1alpha1.V2rayNodeInfo{
			Name:    nodeName,
			Ip:      pubIp,
			Port:    443, //	only support 443 currently
			SvcName: svc.Name,
		})

		//
		successedSvcList = append(successedSvcList, svc)
	}

	//	update k8s v2ray user nodeList data
	v2rayuser.Spec.NodeList = nodeList
	if err := r.Update(ctx, v2rayuser); err != nil {
		log.Error(err, "addUserToV2raySvcList: failed to update v2ray user nodeList")
		return fmt.Errorf("addUserToV2raySvcList: failed to update v2ray user nodeList: %w", err)
	}

	return nil
}

func getNodePublicIp(node *corev1.Node) (string, error) {
	//	get node public address from metadata.annotations.flannel.alpha.coreos.com/public-ip
	pubIp, ok := node.Annotations[constant.FlannelPublicIpAnnotation]
	if ok {
		return pubIp, nil
	}
	pubIp, ok = node.Annotations[constant.K3SInternalIpAnnotation]
	if ok {
		return pubIp, nil
	}
	pubIp, ok = node.Annotations[constant.K8SInternalIpAnnotation]
	if ok {
		return pubIp, nil
	}
	//	get internal ip from status.addresses
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address, nil
		}
	}

	return pubIp, nil
}
