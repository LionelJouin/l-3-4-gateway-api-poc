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

	"github.com/lioneljouin/l-3-4-gateway-api-poc/api/v1alpha1"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/endpointslice"
	v1 "k8s.io/api/core/v1"
	v1discovery "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

type createUpdateEndpointSliceFunc func(
	ctx context.Context,
	addressType v1discovery.AddressType,
	endpointSlice *v1discovery.EndpointSlice,
) error

// reconcileEndpointSlices reconciles the EndpointSlices for IPv4 and IPv6 for a specific service.
func (c *Controller) reconcileEndpointSlices(
	ctx context.Context,
	service *v1.Service,
	pods *v1.PodList,
	networks []*v1alpha1.Network,
) error {
	createUpdateEndpointSliceIPv4Func := c.updateEndpointSlice
	createUpdateEndpointSliceIPv6Func := c.updateEndpointSlice
	ipv4EndpointSlice := &v1discovery.EndpointSlice{}
	ipv6EndpointSlice := &v1discovery.EndpointSlice{}

	// Check if previous endpointslice was existing
	err := c.Get(ctx, types.NamespacedName{
		Name:      endpointslice.GetEndpointSliceName(service, v1discovery.AddressTypeIPv4),
		Namespace: service.GetNamespace(),
	}, ipv4EndpointSlice)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get IPv4 EndpointSlice: %w", err)
		}

		createUpdateEndpointSliceIPv4Func = c.createEndpointSlice
	}

	err = c.Get(ctx, types.NamespacedName{
		Name:      endpointslice.GetEndpointSliceName(service, v1discovery.AddressTypeIPv6),
		Namespace: service.GetNamespace(),
	}, ipv6EndpointSlice)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get IPv6 EndpointSlice: %w", err)
		}

		createUpdateEndpointSliceIPv6Func = c.createEndpointSlice
	}

	// reconcile ipv4 endpointslice
	err = c.reconcileEndpointSlice(
		ctx,
		service,
		pods,
		v1discovery.AddressTypeIPv4,
		createUpdateEndpointSliceIPv4Func,
		networks,
	)
	if err != nil {
		return err
	}

	// reconcile ipv6 endpointslice
	err = c.reconcileEndpointSlice(
		ctx,
		service,
		pods,
		v1discovery.AddressTypeIPv6,
		createUpdateEndpointSliceIPv6Func,
		networks,
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) reconcileEndpointSlice(
	ctx context.Context,
	service *v1.Service,
	pods *v1.PodList,
	addressType v1discovery.AddressType,
	createUpdateEndpointSlice createUpdateEndpointSliceFunc,
	networks []*v1alpha1.Network,
) error {
	endpointSlice, err := endpointslice.GetEndpointSlice(
		service,
		pods,
		addressType,
		networks,
		c.GetIPsFunc,
	)
	if err != nil {
		return fmt.Errorf("failed to reconcile %v EndpointSlice: %w", addressType, err)
	}

	err = ctrl.SetControllerReference(
		service,
		endpointSlice,
		c.Scheme,
	) // todo: what should be the reference (service or gateway)?
	if err != nil {
		return fmt.Errorf("failed to SetControllerReference on EndpointSlice: %w", err)
	}

	err = createUpdateEndpointSlice(ctx, addressType, endpointSlice)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) createEndpointSlice(
	ctx context.Context,
	addressType v1discovery.AddressType,
	endpointSlice *v1discovery.EndpointSlice,
) error {
	err := c.Create(ctx, endpointSlice)
	if err != nil {
		return fmt.Errorf("failed to create %v EndpointSlice: %w", addressType, err)
	}

	return nil
}

func (c *Controller) updateEndpointSlice(
	ctx context.Context,
	addressType v1discovery.AddressType,
	endpointSlice *v1discovery.EndpointSlice,
) error {
	err := c.Update(ctx, endpointSlice)
	if err != nil {
		return fmt.Errorf("failed to update %v EndpointSlice: %w", addressType, err)
	}

	return nil
}
