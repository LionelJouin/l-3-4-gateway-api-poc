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

package cmd

import (
	"context"
	"os"

	"github.com/lioneljouin/l-3-4-gateway-api-poc/api/v1alpha1"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/cli"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/controllers/istio"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/controllers/kpng"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/log"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/networkattachment"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type runOptions struct {
	cli.CommonOptions
	gatewayClassName string
	registry         string
	version          string
}

func newCmdRun() *cobra.Command {
	runOpts := &runOptions{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the istio-controller-manager",
		Long:  `Run the istio-controller-manager`,
		Run: func(cmd *cobra.Command, _ []string) {
			runOpts.run(cmd.Context())
		},
	}

	cmd.Flags().StringVar(
		&runOpts.gatewayClassName,
		"gateway-class-name",
		"",
		"Name of the Gateway Class handled by this controller manager.",
	)

	cmd.Flags().StringVar(
		&runOpts.registry,
		"registry",
		"",
		"registry of the image to use.",
	)
	cmd.Flags().StringVar(
		&runOpts.version,
		"version",
		"",
		"version of the image to use.",
	)

	runOpts.SetCommonFlags(cmd)

	return cmd
}

func (ro *runOptions) run(ctx context.Context) {
	scheme := runtime.NewScheme()
	setupLog := ctrl.Log.WithName("setup")

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(gatewayapiv1.Install(scheme))

	logger := log.New("istio-controller-manager", ro.LogLevel)

	crlog.SetLogger(logger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:         scheme,
		LeaderElection: false,
		Cache:          cache.Options{},
		Metrics: server.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: ":8081",
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: 9443,
			},
		},
	})
	if err != nil {
		log.Fatal(setupLog, "failed to create manager for controllers", "err", err)
	}

	log.FromContextOrGlobal(ctx).Info("Init controller manager", "gatewayClassName", ro.gatewayClassName)

	if err = (&kpng.Controller{
		Client:                   mgr.GetClient(),
		Scheme:                   mgr.GetScheme(),
		GetIPsFunc:               networkattachment.GetIPs,
		GatewayClassName:         ro.gatewayClassName,
		DisabledDaemonSet:        true,
		SetPortsInEndpointSlices: true,
	}).SetupWithManager(mgr); err != nil {
		log.Fatal(setupLog, "failed to create controller", "err", err, "controller", "Gateway")
	}

	if err := (&istio.GatewayMutator{
		Registry: ro.registry,
		Version:  ro.version,
	}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Pod")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatal(setupLog, "unable to set up health check", "err", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Fatal(setupLog, "unable to set up ready check", "err", err)
	}

	if err := mgr.Start(ctx); err != nil {
		log.Fatal(setupLog, "failed to start manager", "err", err)
	}
}
