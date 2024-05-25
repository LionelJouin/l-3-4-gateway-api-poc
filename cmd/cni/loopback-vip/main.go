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

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/lioneljouin/l-3-4-gateway-api-poc/pkg/cni"
	"github.com/vishvananda/netlink"
)

const (
	interfaceName = "lo"
)

// NetConf represents the cni config of the loopback-vip plugin.
type NetConf struct {
	types.NetConf
	Vip string `json:"vip"`

	Args *CNIArgs `json:"args"`
}

// CNIArgs represents CNI_ARGS.
type CNIArgs struct {
	A *EnvArgs `json:"cni"`
}

// EnvArgs represents CNI_ARGS.
type EnvArgs struct {
	types.CommonArgs
	Vip string `json:"vip,omitempty"`
}

//nolint:gochecknoinits
func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func cmdAdd(args *skel.CmdArgs) error {
	config, cniVersion, err := loadConf(args.StdinData)
	if err != nil {
		return err
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %w", args.Netns, err)
	}

	defer netns.Close()

	_, vip, _ := net.ParseCIDR(config.Vip)

	err = netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("failed to lookup %q: %w", interfaceName, err)
		}

		addr := &netlink.Addr{IPNet: vip, Label: ""}
		if err = netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("failed to add IP addr %v to %q: %w", vip, interfaceName, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to apply changes in netns: %w", err)
	}

	err = cni.Store(args)
	if err != nil {
		return fmt.Errorf("failed to store the config in the store: %w", err)
	}

	result := &current.Result{
		CNIVersion: config.CNIVersion,
		Interfaces: []*current.Interface{
			{
				Name:    args.IfName,
				Mac:     "00:00:00:00:00:00",
				Sandbox: args.Netns,
			},
		},
	}

	err = types.PrintResult(result, cniVersion)
	if err != nil {
		return fmt.Errorf("failed to print result: %w", err)
	}

	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	stdinData, err := cni.Get(args)
	if err != nil {
		//nolint:nilerr
		return nil // file does not exist, so the VIP is already removed.
	}

	config, _, err := loadConf(stdinData)
	if err != nil {
		return err
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %w", args.Netns, err)
	}

	defer netns.Close()

	_, vip, _ := net.ParseCIDR(config.Vip)

	err = netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(interfaceName)
		if err != nil {
			return fmt.Errorf("failed to lookup %q: %w", interfaceName, err)
		}

		addr := &netlink.Addr{IPNet: vip, Label: ""}
		if err = netlink.AddrDel(link, addr); err != nil {
			return fmt.Errorf("failed to del IP addr %v to %q: %w", vip, interfaceName, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to apply changes in netns: %w", err)
	}

	err = cni.Delete(args)
	if err != nil {
		return fmt.Errorf("failed to delete the config from the store: %w", err)
	}

	return nil
}

func cmdCheck(_ *skel.CmdArgs) error {
	return nil
}

func cmdGC(_ *skel.CmdArgs) error {
	return nil
}

func main() {
	skel.PluginMainFuncs(skel.CNIFuncs{
		Add:   cmdAdd,
		Check: cmdCheck,
		Del:   cmdDel,
		GC:    cmdGC,
	}, version.All, bv.BuildString("loopback-vip"))
}

func loadConf(stdinData []byte) (*NetConf, string, error) {
	conf := &NetConf{}

	if err := json.Unmarshal(stdinData, conf); err != nil {
		return conf, "", fmt.Errorf("failed to load netconf: %w", err)
	}

	if conf.Args != nil && conf.Args.A != nil && conf.Args.A.Vip != "" {
		conf.Vip = conf.Args.A.Vip
	}

	if _, _, err := net.ParseCIDR(conf.Vip); err != nil {
		return conf, "", fmt.Errorf("wrong vip address format %q: %w", conf.Vip, err)
	}

	return conf, conf.CNIVersion, nil
}
