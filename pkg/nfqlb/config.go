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

package nfqlb

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/log"
)

type nfqlbConfig struct {
	queue          string
	qlength        uint
	fanout         bool
	healInterval   time.Duration
	startingOffset int
	nfqlbPath      string
	logger         logr.Logger
}

func newNFQLBConfig() *nfqlbConfig {
	return &nfqlbConfig{
		queue:          defaultQueue,
		qlength:        defaultQLength,
		fanout:         false,
		healInterval:   defaultHealInterval,
		startingOffset: defaultStartingOffset,
		nfqlbPath:      nfqlbCmd,
		logger:         log.Logger.WithValues("class", "nfqlb"),
	}
}

type nfqlbServiceConfig struct {
	maxTargets int
}

func newNFQLBServiceConfig() *nfqlbServiceConfig {
	return &nfqlbServiceConfig{
		maxTargets: defaultMaxTargets,
	}
}

func (sc *nfqlbServiceConfig) getM() int {
	return sc.maxTargets * maglevMMultiplier
}
