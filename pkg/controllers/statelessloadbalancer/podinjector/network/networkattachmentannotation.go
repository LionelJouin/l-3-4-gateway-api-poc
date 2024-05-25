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

package network

import (
	"encoding/json"
	"fmt"
	"hash/fnv"

	netdefv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/cni"
	v1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/proxy/apis"
)

const (
	vipNADName         = "vip"
	policyRouteNADName = "policy-route"
)

// SetNetworkAttachmentAnnotation modifies the network attachment annotation of the pod to correspond to
// the stateless load balancer requirement (VIP + policy routes) with the parameters (VIPs, Gateways).
func SetNetworkAttachmentAnnotation(
	pod *v1.Pod,
	serviceProxy string,
	vipV4 []string,
	vipV6 []string,
	gatewaysV4 []string,
	gatewaysV6 []string,
) (*v1.Pod, error) {
	newPod := pod.DeepCopy()

	var networkSelectionElements []netdefv1.NetworkSelectionElement

	if newPod.GetAnnotations() == nil {
		newPod.Annotations = map[string]string{}
	} else {
		networks, exists := newPod.GetAnnotations()[netdefv1.NetworkAttachmentAnnot]
		if exists {
			err := json.Unmarshal([]byte(networks), &networkSelectionElements)
			if err != nil {
				return nil, fmt.Errorf("failed to json.Unmarshal Network Attachment Annotation: %w", err)
			}
		}
	}

	newNetworkSelectionElements := getNetworkAttachmentAnnotation(
		networkSelectionElements,
		serviceProxy,
		vipV4,
		vipV6,
		gatewaysV4,
		gatewaysV6,
	)

	newNetworkSelectionElementsJSON, err := json.Marshal(newNetworkSelectionElements)
	if err != nil {
		return nil, fmt.Errorf("failed to json.Marshal newNetworkSelectionElements: %w", err)
	}

	newPod.GetAnnotations()[netdefv1.NetworkAttachmentAnnot] = string(newNetworkSelectionElementsJSON)

	return newPod, nil
}

func getNetworkAttachmentAnnotation(
	networkSelectionElements []netdefv1.NetworkSelectionElement,
	serviceProxy string,
	vipV4 []string,
	vipV6 []string,
	gatewaysV4 []string,
	gatewaysV6 []string,
) []netdefv1.NetworkSelectionElement {
	newNetworkSelectionElements := []netdefv1.NetworkSelectionElement{}
	managedNetworkSelectionElements := []netdefv1.NetworkSelectionElement{}

	// retrieve unmanaged elements
	for _, networkSelectionElement := range networkSelectionElements {
		if networkSelectionElement.CNIArgs != nil {
			proxy, exists := (*networkSelectionElement.CNIArgs)[apis.LabelServiceProxyName]
			if exists && proxy == serviceProxy {
				managedNetworkSelectionElements = append(managedNetworkSelectionElements, networkSelectionElement)

				continue
			}
		}

		newNetworkSelectionElements = append(newNetworkSelectionElements, networkSelectionElement)
	}

	if equalVIPsGateways(managedNetworkSelectionElements,
		vipV4,
		vipV6,
		gatewaysV4,
		gatewaysV6) {
		return networkSelectionElements
	}

	tableID := getFreeTableID(networkSelectionElements)
	policyRoutesV4 := []cni.PolicyRoute{}
	policyRoutesV6 := []cni.PolicyRoute{}

	for _, vip := range vipV4 {
		cniArgsVIP := map[string]any{
			apis.LabelServiceProxyName: serviceProxy,
			"vip":                      vip,
		}

		newNetworkSelectionElements = append(newNetworkSelectionElements, netdefv1.NetworkSelectionElement{
			Name:             vipNADName,
			InterfaceRequest: getHashedInterfaceName(cniArgsVIP),
			CNIArgs:          &cniArgsVIP,
		})

		policyRoutesV4 = append(policyRoutesV4, cni.PolicyRoute{
			SrcPrefix: vip,
		})
	}

	for _, vip := range vipV6 {
		cniArgsVIP := map[string]any{
			apis.LabelServiceProxyName: serviceProxy,
			"vip":                      vip,
		}

		newNetworkSelectionElements = append(newNetworkSelectionElements, netdefv1.NetworkSelectionElement{
			Name:             vipNADName,
			InterfaceRequest: getHashedInterfaceName(cniArgsVIP),
			CNIArgs:          &cniArgsVIP,
		})

		policyRoutesV6 = append(policyRoutesV6, cni.PolicyRoute{
			SrcPrefix: vip,
		})
	}

	if len(gatewaysV4) > 0 && len(policyRoutesV4) > 0 {
		cniArgsPolicyRoute := map[string]any{
			apis.LabelServiceProxyName: serviceProxy,
			"tableId":                  tableID,
			"gateways":                 gatewaysV4,
			"policyRoutes":             policyRoutesV4,
		}

		newNetworkSelectionElements = append(newNetworkSelectionElements, netdefv1.NetworkSelectionElement{
			Name:             policyRouteNADName,
			InterfaceRequest: getHashedInterfaceName(cniArgsPolicyRoute),
			CNIArgs:          &cniArgsPolicyRoute,
		})
	}

	if len(gatewaysV6) > 0 && len(policyRoutesV6) > 0 {
		cniArgsPolicyRoute := map[string]any{
			apis.LabelServiceProxyName: serviceProxy,
			"tableId":                  tableID,
			"gateways":                 gatewaysV6,
			"policyRoutes":             policyRoutesV6,
		}

		newNetworkSelectionElements = append(newNetworkSelectionElements, netdefv1.NetworkSelectionElement{
			Name:             policyRouteNADName,
			InterfaceRequest: getHashedInterfaceName(cniArgsPolicyRoute),
			CNIArgs:          &cniArgsPolicyRoute,
		})
	}

	return newNetworkSelectionElements
}

func getFreeTableID(
	networkSelectionElements []netdefv1.NetworkSelectionElement,
) int {
	tableID := 5000
	usedTableIDs := map[int]struct{}{}

	for _, networkSelectionElement := range networkSelectionElements {
		if networkSelectionElement.CNIArgs == nil || networkSelectionElement.Name != policyRouteNADName {
			continue
		}

		currentTableID, exists := (*networkSelectionElement.CNIArgs)["tableId"]
		if !exists {
			continue
		}

		currentTableIDInt, ok := currentTableID.(float64)
		if !ok {
			continue
		}

		usedTableIDs[int(currentTableIDInt)] = struct{}{}
	}

	for i := 0; i < len(usedTableIDs); i++ {
		_, exists := usedTableIDs[tableID]
		if !exists {
			return tableID
		}

		tableID++
	}

	return tableID
}

func equalVIPsGateways(
	networkSelectionElements []netdefv1.NetworkSelectionElement,
	vipV4 []string,
	vipV6 []string,
	gatewaysV4 []string,
	gatewaysV6 []string,
) bool {
	currentVIPs, currentGateways := getVIPsGatewaysFromNetworkSelectionElements(networkSelectionElements)

	if len(currentVIPs) != len(vipV4)+len(vipV6) || len(currentGateways) != len(gatewaysV4)+len(gatewaysV6) {
		return false
	}

	vips := append([]string{}, vipV4...)
	vips = append(vips, vipV6...)
	gateways := append([]string{}, gatewaysV4...)
	gateways = append(gateways, gatewaysV6...)

	return equalMapSlice(currentVIPs, vips) && equalMapSlice(currentGateways, gateways)
}

func equalMapSlice(comparedMap map[string]struct{}, comparedSlice []string) bool {
	sliceMap := map[string]struct{}{}

	for _, sliceElement := range comparedSlice {
		_, exists := comparedMap[sliceElement]
		if !exists {
			return false
		}

		sliceMap[sliceElement] = struct{}{}
	}

	return len(sliceMap) == len(comparedMap)
}

func getVIPsGatewaysFromNetworkSelectionElements(
	networkSelectionElements []netdefv1.NetworkSelectionElement,
) (map[string]struct{}, map[string]struct{}) {
	vips := map[string]struct{}{}
	gateways := map[string]struct{}{}

	for _, networkSelectionElement := range networkSelectionElements {
		if networkSelectionElement.CNIArgs == nil {
			continue
		}

		switch networkSelectionElement.Name {
		case vipNADName:
			vip, exists := (*networkSelectionElement.CNIArgs)["vip"]
			if !exists {
				continue
			}

			vipString, ok := vip.(string)
			if !ok {
				continue
			}

			vips[vipString] = struct{}{}
		case policyRouteNADName:
			gatewaysInterface, exists := (*networkSelectionElement.CNIArgs)["gateways"]
			if !exists {
				continue
			}

			gatewayInterfaceSlice, ok := gatewaysInterface.([]any)
			if !ok {
				continue
			}

			for _, gateway := range gatewayInterfaceSlice {
				gw, ok := gateway.(string)
				if ok {
					gateways[gw] = struct{}{}
				}
			}
		}
	}

	return vips, gateways
}

func getHashedInterfaceName(data any) string {
	dataJSON, err := json.Marshal(data)
	if err != nil { // Should never happen
		return ""
	}

	h := fnv.New64a()

	h.Write(dataJSON)

	sum := h.Sum(nil)

	// remove 1 character since 16 characters is too long for an interface name.
	return fmt.Sprintf("%x", sum)[1:]
}
