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

package controllermanager

import (
	"context"
	"fmt"

	"github.com/lioneljouin/l-3-4-gateway-api-poc/api/v1alpha1"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/networkattachment"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/proxy/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// reconcileServices reconciles all services managed by the gateway.
func (c *Controller) reconcileServices(ctx context.Context, gateway *gatewayapiv1.Gateway) error {
	networks := networkattachment.GetNetworksFromGateway(gateway)

	services := &v1.ServiceList{}

	// Get pods for this service so the endpointslices can be reconciled.
	matchingLabels := client.MatchingLabels{
		apis.LabelServiceProxyName: gateway.Name,
	}

	err := c.List(ctx, services, matchingLabels) // todo: filter namespace
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	for _, service := range services.Items {
		s := service

		err = c.reconcileService(ctx, &s, networks)
		if err != nil {
			return err
		}
	}

	// todo: cleanup old endpointslices

	return nil
}

// reconcileService reconciles a specific service.
func (c *Controller) reconcileService(ctx context.Context, service *v1.Service, networks []*v1alpha1.Network) error {
	// Get pods for this service so the endpointslices can be reconciled.
	var matchingLabels client.MatchingLabels = service.Spec.Selector

	delete(matchingLabels, v1alpha1.LabelDummmySericeSelector)

	pods := &v1.PodList{}

	err := c.List(ctx,
		pods,
		matchingLabels) // todo: filter namespace
	if err != nil {
		return fmt.Errorf("failed to list the pods: %w", err)
	}

	return c.reconcileEndpointSlices(ctx, service, pods, networks)
}

func ptrTo[T any](a T) *T {
	return &a
}
