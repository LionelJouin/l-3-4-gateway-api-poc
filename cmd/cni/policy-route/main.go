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

// NetConf represents the cni config of the policy-route plugin.
type NetConf struct {
	types.NetConf
	Gateways     []string          `json:"gateways"`
	TableID      int               `json:"tableId"`
	PolicyRoutes []cni.PolicyRoute `json:"policyRoutes"`

	Args *CNIArgs `json:"args"`
}

// CNIArgs represents CNI_ARGS.
type CNIArgs struct {
	A *EnvArgs `json:"cni"`
}

// EnvArgs represents CNI_ARGS.
type EnvArgs struct {
	types.CommonArgs
	Gateways     []string          `json:"gateways"`
	TableID      int               `json:"tableId"`
	PolicyRoutes []cni.PolicyRoute `json:"policyRoutes"`
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

	nexthopsV4, nexthopsV6 := getNextHops(config.Gateways)

	err = netns.Do(func(_ ns.NetNS) error {
		return configureNetNs(config.TableID, config.PolicyRoutes, nexthopsV4, nexthopsV6)
	})
	if err != nil {
		return fmt.Errorf("failed to apply changes in netns: %w", err)
	}

	err = cni.Store(args)
	if err != nil {
		return fmt.Errorf("failed to store the config from the store: %w", err)
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

	err = netns.Do(func(_ ns.NetNS) error {
		err = flushRoutingPolicies(config.TableID)
		if err != nil {
			return err
		}

		return flushRoutingTables(config.TableID)
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
	}, version.All, bv.BuildString("policy-route"))
}

func loadConf(stdinData []byte) (*NetConf, string, error) {
	conf := &NetConf{}

	if err := json.Unmarshal(stdinData, conf); err != nil {
		return conf, "", fmt.Errorf("failed to load netconf: %w", err)
	}

	if conf.Args != nil && conf.Args.A != nil {
		conf.Gateways = conf.Args.A.Gateways
		conf.TableID = conf.Args.A.TableID
		conf.PolicyRoutes = conf.Args.A.PolicyRoutes
	}

	return conf, conf.CNIVersion, nil
}

func configureNetNs(
	tableID int,
	policyRoutes []cni.PolicyRoute,
	nexthopsV4 []*netlink.NexthopInfo,
	nexthopsV6 []*netlink.NexthopInfo,
) error {
	err := flushRoutingPolicies(tableID)
	if err != nil {
		return err
	}

	err = flushRoutingTables(tableID)
	if err != nil {
		return err
	}

	if len(nexthopsV4) >= 1 {
		routeV4 := &netlink.Route{
			Table:     tableID,
			Src:       net.IPv4(0, 0, 0, 0),
			MultiPath: nexthopsV4,
		}

		err := netlink.RouteAdd(routeV4)
		if err != nil {
			return fmt.Errorf("failed to add ipv4 route: %w", err)
		}
	}

	if len(nexthopsV6) >= 1 {
		routeV6 := &netlink.Route{
			Table:     tableID,
			Src:       net.ParseIP("::"),
			MultiPath: nexthopsV6,
		}

		err = netlink.RouteAdd(routeV6)
		if err != nil {
			return fmt.Errorf("failed to add ipv6 route: %w", err)
		}
	}

	for _, policyRoute := range policyRoutes {
		_, srcPrefix, err := net.ParseCIDR(policyRoute.SrcPrefix)
		if err != nil {
			continue
		}

		rule := netlink.NewRule()
		rule.Table = tableID
		rule.Src = &net.IPNet{
			IP:   srcPrefix.IP,
			Mask: srcPrefix.Mask,
		}

		if srcPrefix.IP.To4() == nil {
			rule.Family = netlink.FAMILY_V6
		} else {
			rule.Family = netlink.FAMILY_V4
		}

		err = netlink.RuleAdd(rule)
		if err != nil {
			return fmt.Errorf("failed to add rule: %w", err)
		}
	}

	return nil
}

func getNextHops(gateways []string) ([]*netlink.NexthopInfo, []*netlink.NexthopInfo) {
	nexthopsV4 := []*netlink.NexthopInfo{}
	nexthopsV6 := []*netlink.NexthopInfo{}

	for _, gateway := range gateways {
		nexthop := net.ParseIP(gateway)
		if len(nexthop) == 0 {
			continue
		}

		if nexthop.To4() == nil {
			nexthopsV6 = append(nexthopsV6, &netlink.NexthopInfo{
				Gw: nexthop,
			})

			continue
		}

		nexthopsV4 = append(nexthopsV4, &netlink.NexthopInfo{
			Gw: nexthop,
		})
	}

	return nexthopsV4, nexthopsV6
}

func flushRoutingPolicies(tableID int) error {
	rules, err := netlink.RuleListFiltered(netlink.FAMILY_ALL, &netlink.Rule{
		Table: tableID,
	}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return fmt.Errorf("failed to list rules: %w", err)
	}

	for _, rule := range rules {
		currentRule := rule

		err = netlink.RuleDel(&currentRule)
		if err != nil {
			return fmt.Errorf("failed to delete rules: %w", err)
		}
	}

	return nil
}

func flushRoutingTables(tableID int) error {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		Table: tableID,
	}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	for _, route := range routes {
		currentRoute := route

		err = netlink.RouteDel(&currentRoute)
		if err != nil {
			return fmt.Errorf("failed to delete route: %w", err)
		}
	}

	return nil
}
