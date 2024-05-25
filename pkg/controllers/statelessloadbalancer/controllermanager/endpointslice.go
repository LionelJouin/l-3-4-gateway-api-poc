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
	"strconv"

	"github.com/lioneljouin/l-3-4-gateway-api-poc/api/v1alpha1"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/controllers/statelessloadbalancer/endpoint"
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
	newIPV4EndpointSlice, err := endpointslice.GetEndpointSlice(
		service,
		pods,
		v1discovery.AddressTypeIPv4,
		networks,
		c.GetIPsFunc,
	)
	if err != nil {
		return fmt.Errorf("failed to reconcile %v EndpointSlice: %w", v1discovery.AddressTypeIPv4, err)
	}

	// reconcile ipv6 endpointslice
	newIPV6EndpointSlice, err := endpointslice.GetEndpointSlice(
		service,
		pods,
		v1discovery.AddressTypeIPv6,
		networks,
		c.GetIPsFunc,
	)
	if err != nil {
		return fmt.Errorf("failed to reconcile %v EndpointSlice: %w", v1discovery.AddressTypeIPv6, err)
	}

	return c.reconcileEndpointSlice(
		ctx,
		service,
		ipv4EndpointSlice,
		ipv6EndpointSlice,
		newIPV4EndpointSlice,
		newIPV6EndpointSlice,
		createUpdateEndpointSliceIPv4Func,
		createUpdateEndpointSliceIPv6Func,
	)
}

func (c *Controller) reconcileEndpointSlice(
	ctx context.Context,
	service *v1.Service,
	oldIPV4EndpointSlice *v1discovery.EndpointSlice,
	oldIPV6EndpointSlice *v1discovery.EndpointSlice,
	newIPV4EndpointSlice *v1discovery.EndpointSlice,
	newIPV6EndpointSlice *v1discovery.EndpointSlice,
	createUpdateEndpointSliceIPv4 createUpdateEndpointSliceFunc,
	createUpdateEndpointSliceIPv6 createUpdateEndpointSliceFunc,
) error {
	var maxEndpoints uint32 = 100 // default max endpoints

	valueServiceMaxEndpoints, exists := service.GetLabels()[v1alpha1.LabelServiceMaxEndpoints]
	if exists {
		maxEndpointsInt, err := strconv.Atoi(valueServiceMaxEndpoints)
		if err == nil {
			maxEndpoints = uint32(maxEndpointsInt)
		}
	}

	finalIPV4EndpointSlice, finalIPV6EndpointSlice := getEndpointSlicesWithIdentifiers(
		oldIPV4EndpointSlice,
		oldIPV6EndpointSlice,
		newIPV4EndpointSlice,
		newIPV6EndpointSlice,
		maxEndpoints,
	)

	err := ctrl.SetControllerReference(
		service,
		finalIPV4EndpointSlice,
		c.Scheme,
	) // todo: what should be the reference (service or gateway)?
	if err != nil {
		return fmt.Errorf("failed to SetControllerReference on EndpointSlice: %w", err)
	}

	err = ctrl.SetControllerReference(
		service,
		finalIPV6EndpointSlice,
		c.Scheme,
	) // todo: what should be the reference (service or gateway)?
	if err != nil {
		return fmt.Errorf("failed to SetControllerReference on EndpointSlice: %w", err)
	}

	err = createUpdateEndpointSliceIPv4(ctx, v1discovery.AddressTypeIPv4, finalIPV4EndpointSlice)
	if err != nil {
		return err
	}

	err = createUpdateEndpointSliceIPv6(ctx, v1discovery.AddressTypeIPv6, finalIPV6EndpointSlice)
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

func getEndpointSlicesWithIdentifiers(
	oldIPV4EndpointSlice *v1discovery.EndpointSlice,
	oldIPV6EndpointSlice *v1discovery.EndpointSlice,
	newIPV4EndpointSlice *v1discovery.EndpointSlice,
	newIPV6EndpointSlice *v1discovery.EndpointSlice,
	maxEndpoints uint32,
) (*v1discovery.EndpointSlice, *v1discovery.EndpointSlice) {
	oldEndpointSlice := endpointslice.MergeEndpointSlices(oldIPV4EndpointSlice, oldIPV6EndpointSlice)
	newEndpointSlice := endpointslice.MergeEndpointSlices(newIPV4EndpointSlice, newIPV6EndpointSlice)
	finalEndpointSlice := &v1discovery.EndpointSlice{
		Endpoints: []v1discovery.Endpoint{},
	}

	endpointIdentifier := map[string]int{}
	identifierInUse := map[int]struct{}{}
	newEndpointsMap := map[string]struct{}{}

	for _, endpnt := range newEndpointSlice.Endpoints {
		newEndpointsMap[string(endpnt.TargetRef.UID)] = struct{}{}
	}

	for _, endpnt := range oldEndpointSlice.Endpoints {
		id := endpoint.GetIdentifier(endpnt)
		if id == nil {
			continue
		}

		_, exists := newEndpointsMap[string(endpnt.TargetRef.UID)]
		if !exists {
			continue
		}

		endpointIdentifier[string(endpnt.TargetRef.UID)] = *id
		identifierInUse[*id] = struct{}{}
	}

	for _, endpnt := range newEndpointSlice.Endpoints {
		id, exists := endpointIdentifier[string(endpnt.TargetRef.UID)]
		if exists {
			finalEndpointSlice.Endpoints = append(finalEndpointSlice.Endpoints, *endpoint.SetIdentifier(endpnt, id))

			continue
		}

		id = getIdentifier(identifierInUse, maxEndpoints)
		if id < 0 {
			continue
		}

		identifierInUse[id] = struct{}{}
		finalEndpointSlice.Endpoints = append(finalEndpointSlice.Endpoints, *endpoint.SetIdentifier(endpnt, id))
	}

	finalIPV4EndpointSlice, finalIPV6EndpointSlice := endpointslice.SplitEndpointSlices(finalEndpointSlice)
	finalIPV4EndpointSlice.ObjectMeta = newIPV4EndpointSlice.ObjectMeta
	finalIPV6EndpointSlice.ObjectMeta = newIPV6EndpointSlice.ObjectMeta

	return finalIPV4EndpointSlice, finalIPV6EndpointSlice
}

// getIdentifier returns a free identifier. -1 is returned if none could be found.
func getIdentifier(identifierInUseMap map[int]struct{}, maxEndpoints uint32) int {
	for i := 0; i < int(maxEndpoints); i++ {
		_, exists := identifierInUseMap[i]
		if !exists {
			return i
		}
	}

	return -1
}
