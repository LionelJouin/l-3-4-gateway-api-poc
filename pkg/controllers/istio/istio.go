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

package istio

import (
	"context"
	"fmt"

	"github.com/lioneljouin/l-3-4-gateway-api-poc/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
)

const GatewayAPILabel = "gateway.networking.k8s.io/gateway-name"

type GatewayMutator struct {
	Registry string
	Version  string
}

func (gm *GatewayMutator) Default(ctx context.Context, obj runtime.Object) error {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		return nil
	}

	_, exists := pod.GetLabels()[v1alpha1.LabelKPNGInject]
	if !exists {
		return nil
	}

	gatewayName, exists := pod.GetLabels()[GatewayAPILabel]
	if !exists {
		return nil
	}

	pod.Spec.Containers = append(pod.Spec.Containers, gm.getRouter(gatewayName, pod.GetNamespace()))
	pod.Spec.Containers = append(pod.Spec.Containers, gm.getKPNG(gatewayName)...)
	pod.Spec.Volumes = append(pod.Spec.Volumes, getVolumes()...)

	return nil
}

func (gm *GatewayMutator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&v1.Pod{}).
		WithDefaulter(gm).
		Complete()
}

func (gm *GatewayMutator) getKPNG(gatewayName string) []v1.Container {
	t := true

	return []v1.Container{
		{
			Name:            "kpng",
			Image:           "ghcr.io/lioneljouin/l-3-4-gateway-api-poc/kpng:latest",
			ImagePullPolicy: v1.PullAlways,
			Args: []string{
				"kube",
				"to-api",
				"--exportMetrics=0.0.0.0:9099",
				fmt.Sprintf("--service-proxy-name=%s", gatewayName),
				"--v=2",
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "empty",
					MountPath: "k8s",
				},
				{
					Name:      "kpng-config",
					MountPath: "/var/lib/kpng",
				},
			},
		},
		{
			Name:            "kpng-ipvs",
			Image:           "ghcr.io/lioneljouin/l-3-4-gateway-api-poc/kpng:latest",
			ImagePullPolicy: v1.PullAlways,
			Args: []string{
				"local",
				"to-ipvs",
				"--exportMetrics=0.0.0.0:9098",
				"--masquerade-all",
				"--v=2",
			},
			SecurityContext: &v1.SecurityContext{
				Privileged: &t,
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "empty",
					MountPath: "k8s",
				},
				{
					Name:      "modules",
					MountPath: "/lib/modules",
					ReadOnly:  true,
				},
			},
		},
	}
}

func (gm *GatewayMutator) getRouter(gatewayName string, namespace string) v1.Container {
	t := true

	return v1.Container{
		Name:            "router",
		Image:           fmt.Sprintf("%s/router:%s", gm.Registry, gm.Version),
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"./router"},
		Args: []string{
			"run",
			fmt.Sprintf("--name=%s", gatewayName),
			fmt.Sprintf("--namespace=%s", namespace),
		},
		SecurityContext: &v1.SecurityContext{
			Privileged: &t,
		},
		VolumeMounts: []v1.VolumeMount{
			{
				MountPath: "/tmp",
				Name:      "tmp",
			},
			{
				MountPath: "/var/run/bird",
				Name:      "run",
			},
			{
				MountPath: "/etc/bird",
				Name:      "etc",
			},
			{
				MountPath: "/var/log",
				Name:      "log",
			},
		},
	}
}

func getVolumes() []v1.Volume {
	return []v1.Volume{
		{
			Name: "empty",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "modules",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/lib/modules",
				},
			},
		},
		{
			Name: "kpng-config",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "kpng",
					},
				},
			},
		},
		{
			Name: "tmp",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
		{
			Name: "run",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
		{
			Name: "etc",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
		{
			Name: "log",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
	}
}
