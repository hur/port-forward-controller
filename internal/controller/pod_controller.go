/*
Copyright 2025.

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

	"atte.cloud/port-forward-controller/internal/forwarding"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const Annotation = "port-forward-controller.atte.cloud"

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Fwd    *forwarding.ForwardingReconciler
}

// +kubebuilder:rbac:groups=atte.cloud,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=atte.cloud,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=atte.cloud,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.4/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var pod v1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to get pod")
		return ctrl.Result{}, err
	}

	if !r.controls(&pod) {
		return ctrl.Result{}, nil
	}

	hostPorts := []forwarding.PortForward{}
	for _, container := range pod.Spec.Containers {
		// TODO: consider indexed access to the slice to be copy-free
		//var nodeName string
		for _, port := range container.Ports {
			if port.HostPort == 0 {
				continue
			}

			info := forwarding.PortForward{
				Address: pod.Status.HostIP,
				Port:    port.HostPort,
				Name:    fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
			}

			hostPorts = append(hostPorts, info)
			//nodeName = pod.Spec.NodeName
		}
		//node := &v1.Node{}
		//if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
		//	log.Error(err, "Unable to get node")
		//	return ctrl.Result{}, err
		//}

		//node.Status.Addresses
	}

	finalizerName := fmt.Sprintf("finalizer.%s/v1", Annotation)

	if pod.ObjectMeta.DeletionTimestamp.IsZero() {
		err := r.Fwd.EnsureAddresses(ctx, hostPorts)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !controllerutil.ContainsFinalizer(&pod, finalizerName) {
			controllerutil.AddFinalizer(&pod, finalizerName)
			if err = r.Update(ctx, &pod); err != nil {
				return ctrl.Result{}, err
			}
		}

	} else {
		err := r.Fwd.DeleteAddresses(ctx, hostPorts)
		if err != nil {
			return ctrl.Result{}, err
		}

		if controllerutil.ContainsFinalizer(&pod, finalizerName) {
			controllerutil.RemoveFinalizer(&pod, finalizerName)
			if err = r.Update(ctx, &pod); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	log.Info("Reconcile successful")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&v1.Pod{}).
		Named("pod").
		Complete(r)
}

func (r *PodReconciler) controls(pod *v1.Pod) bool {
	for annotation, value := range pod.GetAnnotations() {
		if annotation == fmt.Sprintf("%s/enable", Annotation) && value == "true" {
			return true
		}
	}
	return false
}

func hasFinalizer(pod *v1.Pod, name string) bool {
	for _, finalizer := range pod.GetObjectMeta().GetFinalizers() {
		if finalizer == name {
			return true
		}
	}
	return false
}
