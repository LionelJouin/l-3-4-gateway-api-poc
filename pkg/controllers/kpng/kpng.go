/*
Copyright (c) 2024 OpenInfra Foundation Europe

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

package kpng

import (
	"context"
	"fmt"

	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/endpointslice"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/log"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v1discovery "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// Controller reconciles the Gateway Object to run KPNG.
type Controller struct {
	client.Client
	Scheme *runtime.Scheme
	// GetIPsFunc is used when the endpointSlice will be reconciled to get the IPs
	// of the pods attached to the service.
	GetIPsFunc       endpointslice.GetIPs
	GatewayClassName string
}

// Reconcile implements the reconciliation of the Gateway of KPNG class.
// This function is trigger by any change (create/update/delete) in any resource related
// to the object (Service/Gateway/EndpointSlice/Pod/DaemonSet).
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gateway := &gatewayapiv1.Gateway{}

	err := c.Get(ctx, req.NamespacedName, gateway)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get the gateway: %w", err)
	}

	if string(gateway.Spec.GatewayClassName) != c.GatewayClassName {
		// this should not happen if the controller is configured correctly.
		return ctrl.Result{}, nil
	}

	err = c.reconcileKPNGDaemonSet(ctx, gateway)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile the kpng daemonset: %w", err)
	}

	log.FromContextOrGlobal(ctx).Info("KPNG Daemonset reconciled")

	err = c.reconcileServices(ctx, gateway)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile the kpng daemonset: %w", err)
	}

	log.FromContextOrGlobal(ctx).Info("KPNG services and endpointslices reconciled")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&gatewayapiv1.Gateway{}).
		// With EnqueueRequestsFromMapFunc, on an update the func is called twice
		// (1 time for old and 1 time for new object)
		Owns(&appsv1.DaemonSet{}).
		Owns(&v1discovery.EndpointSlice{}).
		Watches(&v1.Pod{}, handler.EnqueueRequestsFromMapFunc(c.podEnqueue)).
		Watches(&v1.Service{}, handler.EnqueueRequestsFromMapFunc(serviceEnqueue)).
		Complete(c)
	if err != nil {
		return fmt.Errorf("failed to build the kpng manager: %w", err)
	}

	return nil
}
