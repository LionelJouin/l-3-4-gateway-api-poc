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

	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/template"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	label               = "app"
	routerContainerName = "router"
	kpngContainerName   = "kpng"
)

func (c *Controller) reconcileKPNGDaemonSet(
	ctx context.Context,
	gateway *gatewayapiv1.Gateway,
) error {
	knpgDaemonSet := &appsv1.DaemonSet{}

	knpgDaemonSetLatestState, err := c.getKPNGDaemonSet(gateway)
	if err != nil {
		return err
	}

	err = c.Get(ctx, types.NamespacedName{
		Name:      getKPNGDaemonSetName(gateway),
		Namespace: gateway.Namespace,
	}, knpgDaemonSet)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create
			err = c.Create(ctx, knpgDaemonSetLatestState)
			if err != nil {
				return fmt.Errorf("failed to create the kpng daemonset: %w", err)
			}

			return nil
		}

		return fmt.Errorf("failed to get the kpng daemonset: %w", err)
	}

	// Update
	err = c.Update(ctx, knpgDaemonSetLatestState)
	if err != nil {
		return fmt.Errorf("failed to update the kpng daemonset: %w", err)
	}

	return nil
}

func (c *Controller) getKPNGDaemonSet(
	gateway *gatewayapiv1.Gateway,
	// templateProvider TemplateProvider,
) (*appsv1.DaemonSet, error) {
	daemonSet := &appsv1.DaemonSet{}

	err := template.Read("/templates/kpng.yaml", daemonSet)
	if err != nil {
		return nil, fmt.Errorf("failed to read daemonset kpng template: %w", err)
	}

	name := getKPNGDaemonSetName(gateway)

	daemonSet.ObjectMeta.Name = name
	daemonSet.ObjectMeta.Namespace = gateway.Namespace

	if daemonSet.Labels == nil {
		daemonSet.Labels = map[string]string{}
	}

	daemonSet.Labels[label] = name

	daemonSet.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			label: name,
		},
	}

	if daemonSet.Spec.Template.Labels == nil {
		daemonSet.Spec.Template.Labels = map[string]string{}
	}

	daemonSet.Spec.Template.Labels[label] = name

	for key, value := range gateway.Spec.Infrastructure.Labels {
		daemonSet.Labels[string(key)] = string(value)
		daemonSet.Spec.Template.Labels[string(key)] = string(value)
	}

	if daemonSet.Annotations == nil {
		daemonSet.Annotations = map[string]string{}
	}

	if daemonSet.Spec.Template.Annotations == nil {
		daemonSet.Spec.Template.Annotations = map[string]string{}
	}

	for key, value := range gateway.Spec.Infrastructure.Annotations {
		daemonSet.Annotations[string(key)] = string(value)
		daemonSet.Spec.Template.Annotations[string(key)] = string(value)
	}

	for index, container := range daemonSet.Spec.Template.Spec.Containers {
		switch container.Name {
		case kpngContainerName:
			daemonSet.Spec.Template.Spec.Containers[index].Args = append(daemonSet.Spec.Template.Spec.Containers[index].Args,
				fmt.Sprintf("--service-proxy-name=%s", gateway.Name),
			)
		case routerContainerName:
			daemonSet.Spec.Template.Spec.Containers[index].Args = append(daemonSet.Spec.Template.Spec.Containers[index].Args,
				fmt.Sprintf("--name=%s", gateway.Name),
				fmt.Sprintf("--namespace=%s", gateway.Namespace),
			)
		}
	}

	err = ctrl.SetControllerReference(gateway, daemonSet, c.Scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to SetControllerReference on kpng daemonset: %w", err)
	}

	return daemonSet, nil
}

func getKPNGDaemonSetName(gateway *gatewayapiv1.Gateway) string {
	return fmt.Sprintf("kpng-%s", gateway.Name)
}
