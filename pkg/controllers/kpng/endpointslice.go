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
	"net"
	"strings"

	"github.com/lioneljouin/l-3-4-gateway-api-poc/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	v1discovery "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		Name:      getEndpointSliceName(service, v1discovery.AddressTypeIPv4),
		Namespace: service.GetNamespace(),
	}, ipv4EndpointSlice)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get IPv4 EndpointSlice: %w", err)
		}

		createUpdateEndpointSliceIPv4Func = c.createEndpointSlice
	}

	err = c.Get(ctx, types.NamespacedName{
		Name:      getEndpointSliceName(service, v1discovery.AddressTypeIPv6),
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
	endpointSlice, err := c.getEndpointSlice(
		service,
		pods,
		addressType,
		networks,
	)
	if err != nil {
		return fmt.Errorf("failed to reconcile %v EndpointSlice: %w", addressType, err)
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

func (c *Controller) getEndpointSlice(
	service *v1.Service,
	pods *v1.PodList,
	addressType v1discovery.AddressType,
	networks []*v1alpha1.Network,
) (*v1discovery.EndpointSlice, error) {
	endpoints := c.getEndpoints(pods, addressType, networks)

	endpointSlice := &v1discovery.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getEndpointSliceName(service, addressType),
			Namespace: service.GetNamespace(),
			Labels: map[string]string{
				v1discovery.LabelServiceName: service.GetName(),
			},
		},
		Endpoints:   endpoints,
		AddressType: addressType,
	}

	err := ctrl.SetControllerReference(service, endpointSlice, c.Scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to SetControllerReference on EndpointSlice: %w", err)
	}

	return endpointSlice, nil
}

func (c *Controller) getEndpoints(
	pods *v1.PodList,
	addressType v1discovery.AddressType,
	networks []*v1alpha1.Network,
) []v1discovery.Endpoint {
	endpoints := []v1discovery.Endpoint{}

	// get the IPs and readiness of the pods
	for _, pod := range pods.Items {
		ready := podReady(pod)

		ips, _ := c.GetIPsFunc(pod, networks) // todo: error

		endpointIPs := []string{}

		// Get only IPs of correct IP family
		for _, ip := range ips {
			ipAddr := net.ParseIP(ip)

			if ipAddr == nil {
				continue
			}

			ipFamily := v1discovery.AddressTypeIPv6

			if ipAddr.To4() != nil {
				ipFamily = v1discovery.AddressTypeIPv4
			}

			if ipFamily == addressType {
				endpointIPs = append(endpointIPs, ip)
			}
		}

		// an endpoint does not access empty address list
		if len(endpointIPs) == 0 {
			continue
		}

		endpnt := v1discovery.Endpoint{
			TargetRef: &v1.ObjectReference{
				Kind:      pod.Kind,
				Name:      pod.GetName(),
				Namespace: pod.GetNamespace(),
				UID:       pod.GetUID(),
			},
			Conditions: v1discovery.EndpointConditions{
				Ready: &ready,
			},
			Addresses: endpointIPs,
		}
		endpoints = append(endpoints, endpnt)
	}

	return endpoints
}

// podReady checks if a pod is ready: All containers in ready state and pod status running.
func podReady(pod v1.Pod) bool {
	for _, container := range pod.Status.ContainerStatuses {
		if !container.Ready {
			return false
		}
	}

	return pod.Status.Phase == v1.PodRunning
}

// getEndpointSliceName concatenates the service name and the address type (ip family).
func getEndpointSliceName(service *v1.Service, addressType v1discovery.AddressType) string {
	return fmt.Sprintf("%s-%s", service.GetName(), strings.ToLower(string(addressType)))
}
