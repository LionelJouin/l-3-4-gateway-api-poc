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
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	types020 "github.com/containernetworking/cni/pkg/types/020"
	types040 "github.com/containernetworking/cni/pkg/types/040"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"k8s.io/kubernetes/pkg/proxy/apis"
)

type tester interface {
	// verifyResult minimally verifies the Result and returns the interface's MAC address
	verifyResult(result types.Result, name string) string
}

type testerBase struct{}

type (
	testerV10x      testerBase
	testerV04x      testerBase
	testerV03x      testerBase
	testerV01xOr02x testerBase
)

func newTesterByVersion(version string) tester {
	switch {
	case strings.HasPrefix(version, "1.0."):
		return &testerV10x{}
	case strings.HasPrefix(version, "0.4."):
		return &testerV04x{}
	case strings.HasPrefix(version, "0.3."):
		return &testerV03x{}
	default:
		return &testerV01xOr02x{}
	}
}

// verifyResult minimally verifies the Result and returns the interface's MAC address
func (t *testerV10x) verifyResult(result types.Result, name string) string {
	r, err := types100.GetResult(result)
	Expect(err).NotTo(HaveOccurred())

	Expect(r.Interfaces).To(HaveLen(1))
	Expect(r.Interfaces[0].Name).To(Equal(name))

	return r.Interfaces[0].Mac
}

func verify0403(result types.Result, name string) string {
	r, err := types040.GetResult(result)
	Expect(err).NotTo(HaveOccurred())

	Expect(r.Interfaces).To(HaveLen(1))
	Expect(r.Interfaces[0].Name).To(Equal(name))

	return r.Interfaces[0].Mac
}

// verifyResult minimally verifies the Result and returns the interface's MAC address
func (t *testerV04x) verifyResult(result types.Result, name string) string {
	return verify0403(result, name)
}

// verifyResult minimally verifies the Result and returns the interface's MAC address
func (t *testerV03x) verifyResult(result types.Result, name string) string {
	return verify0403(result, name)
}

// verifyResult minimally verifies the Result and returns the interface's MAC address
func (t *testerV01xOr02x) verifyResult(result types.Result, _ string) string {
	r, err := types020.GetResult(result)
	Expect(err).NotTo(HaveOccurred())

	Expect(r.IP4.IP.IP).NotTo(BeNil())
	Expect(r.IP6).To(BeNil())

	// 0.2 and earlier don't return MAC address
	return ""
}

var _ = Describe("policy-route Operations", func() {
	var originalNS, targetNS ns.NetNS
	var dataDir string

	BeforeEach(func() {
		// Create a new NetNS so we don't modify the host
		var err error
		originalNS, err = testutils.NewNS()
		Expect(err).NotTo(HaveOccurred())
		targetNS, err = testutils.NewNS()
		Expect(err).NotTo(HaveOccurred())

		dataDir, err = os.MkdirTemp("", "policy-route_test")
		Expect(err).NotTo(HaveOccurred())

		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			// Add master
			err = netlink.LinkAdd(&netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "test-dummy",
				},
			})
			Expect(err).NotTo(HaveOccurred())
			m, err := netlink.LinkByName("test-dummy")
			Expect(err).NotTo(HaveOccurred())

			addr, err := netlink.ParseAddr("169.255.100.5/24")
			Expect(err).NotTo(HaveOccurred())

			err = netlink.AddrAdd(m, addr)
			Expect(err).NotTo(HaveOccurred())

			err = netlink.LinkSetUp(m)
			Expect(err).NotTo(HaveOccurred())

			m, err = netlink.LinkByName("lo")
			Expect(err).NotTo(HaveOccurred())

			addr, err = netlink.ParseAddr("20.0.0.1/32")
			Expect(err).NotTo(HaveOccurred())

			err = netlink.AddrAdd(m, addr)
			Expect(err).NotTo(HaveOccurred())

			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(dataDir)).To(Succeed())
		Expect(originalNS.Close()).To(Succeed())
		Expect(testutils.UnmountNS(originalNS)).To(Succeed())
		Expect(targetNS.Close()).To(Succeed())
		Expect(testutils.UnmountNS(targetNS)).To(Succeed())
	})

	ver := "1.0.0"

	It(fmt.Sprintf("[%s] configures and deconfigures a ipv4 policy route with ADD/CHECK/DEL", "v1.0.0"), func() {
		const IFNAME = "abc"

		gateway := "169.255.100.1"
		vip := "20.0.0.1/32"
		tableID := 5000

		_, vipNetIP, err := net.ParseCIDR(vip)
		Expect(err).NotTo(HaveOccurred())

		conf := fmt.Sprintf(`{
			"cniVersion": "%s",
			"type": "policy-route",
			"args": {
				"cni": {
					"%s": "proxy-a",
					"gateways": [
						"%s"
					],
					"policyRoutes": [
						{
							"srcPrefix": "%s"
						}
					],
					"tableId": %d
				}
			}
		}`, ver, apis.LabelServiceProxyName, gateway, vip, tableID)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNS.Path(),
			IfName:      IFNAME,
			StdinData:   []byte(conf),
		}

		t := newTesterByVersion(ver)

		var result types.Result

		// Call ADD
		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			var err error
			result, _, err = testutils.CmdAddWithArgs(args, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())

			_ = t.verifyResult(result, IFNAME)

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Make sure policy route exists in the target namespace
		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			routes, err := netlink.RouteListFiltered(
				netlink.FAMILY_V4,
				&netlink.Route{
					Gw:    net.ParseIP(gateway),
					Table: tableID,
				},
				netlink.RT_FILTER_GW|netlink.RT_FILTER_TABLE,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(routes)).To(Equal(1))
			Expect(routes[0].Gw.String()).To(Equal(gateway))
			Expect(routes[0].Table).To(Equal(tableID))

			rule := netlink.NewRule()
			rule.Table = tableID
			rule.Src = vipNetIP
			rule.Family = netlink.FAMILY_V4

			rules, err := netlink.RuleListFiltered(
				netlink.FAMILY_V4,
				rule,
				netlink.RT_FILTER_TABLE|netlink.RT_FILTER_SRC,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rules)).To(Equal(1))
			Expect(rules[0].Src).To(Equal(vipNetIP))
			Expect(rules[0].Table).To(Equal(tableID))

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Call DEL
		// remove args since DEL could be achieved only with CNI_CONTAINERID and CNI_IFNAME
		// https://www.cni.dev/docs/spec/#section-3-execution-of-network-configurations
		conf = fmt.Sprintf(`{
			"cniVersion": "%s",
			"type": "policy-route"
		}`, ver)
		args = &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNS.Path(),
			IfName:      IFNAME,
			StdinData:   []byte(conf),
		}
		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Make sure policy route has been deleted
		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			routes, err := netlink.RouteListFiltered(
				netlink.FAMILY_V4,
				&netlink.Route{
					Gw:    net.ParseIP(gateway),
					Table: tableID,
				},
				netlink.RT_FILTER_GW|netlink.RT_FILTER_TABLE,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(routes)).To(Equal(0))

			rule := netlink.NewRule()
			rule.Table = tableID
			rule.Src = vipNetIP
			rule.Family = netlink.FAMILY_V4

			rules, err := netlink.RuleListFiltered(
				netlink.FAMILY_V4,
				rule,
				netlink.RT_FILTER_TABLE|netlink.RT_FILTER_SRC,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rules)).To(Equal(0))

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// DEL can be called multiple times, make sure no error is returned
		// if the device is already removed.
		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It(fmt.Sprintf("[%s] configures and deconfigures a ipv4 policy route with multiple gateways with ADD/CHECK/DEL", "v1.0.0"), func() {
		const IFNAME = "abc"

		gateway1 := "169.255.100.1"
		gateway2 := "169.255.100.2"
		vip := "20.0.0.1/32"
		tableID := 5000

		_, vipNetIP, err := net.ParseCIDR(vip)
		Expect(err).NotTo(HaveOccurred())

		conf := fmt.Sprintf(`{
			"cniVersion": "%s",
			"type": "policy-route",
			"args": {
				"cni": {
					"%s": "proxy-a",
					"gateways": [
						"%s",
						"%s"
					],
					"policyRoutes": [
						{
							"srcPrefix": "%s"
						}
					],
					"tableId": %d
				}
			}
		}`, ver, apis.LabelServiceProxyName, gateway1, gateway2, vip, tableID)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNS.Path(),
			IfName:      IFNAME,
			StdinData:   []byte(conf),
		}

		t := newTesterByVersion(ver)

		var result types.Result

		// Call ADD
		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			var err error
			result, _, err = testutils.CmdAddWithArgs(args, func() error {
				return cmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())

			_ = t.verifyResult(result, IFNAME)

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Make sure policy route exists in the target namespace
		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			routes, err := netlink.RouteListFiltered(
				netlink.FAMILY_V4,
				&netlink.Route{
					Table: tableID,
				},
				netlink.RT_FILTER_GW|netlink.RT_FILTER_TABLE,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(routes)).To(Equal(1))
			Expect(len(routes[0].MultiPath)).To(Equal(2))
			Expect(routes[0].MultiPath[0].Gw.String()).To(Equal(gateway1))
			Expect(routes[0].MultiPath[1].Gw.String()).To(Equal(gateway2))
			Expect(routes[0].Table).To(Equal(tableID))

			rule := netlink.NewRule()
			rule.Table = tableID
			rule.Src = vipNetIP
			rule.Family = netlink.FAMILY_V4

			rules, err := netlink.RuleListFiltered(
				netlink.FAMILY_V4,
				rule,
				netlink.RT_FILTER_TABLE|netlink.RT_FILTER_SRC,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rules)).To(Equal(1))
			Expect(rules[0].Src).To(Equal(vipNetIP))
			Expect(rules[0].Table).To(Equal(tableID))

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Call DEL
		// remove args since DEL could be achieved only with CNI_CONTAINERID and CNI_IFNAME
		// https://www.cni.dev/docs/spec/#section-3-execution-of-network-configurations
		conf = fmt.Sprintf(`{
			"cniVersion": "%s",
			"type": "policy-route"
		}`, ver)
		args = &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       targetNS.Path(),
			IfName:      IFNAME,
			StdinData:   []byte(conf),
		}
		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Make sure policy route has been deleted
		err = targetNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			routes, err := netlink.RouteListFiltered(
				netlink.FAMILY_V4,
				&netlink.Route{
					Table: tableID,
				},
				netlink.RT_FILTER_GW|netlink.RT_FILTER_TABLE,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(routes)).To(Equal(0))

			rule := netlink.NewRule()
			rule.Table = tableID
			rule.Src = vipNetIP
			rule.Family = netlink.FAMILY_V4

			rules, err := netlink.RuleListFiltered(
				netlink.FAMILY_V4,
				rule,
				netlink.RT_FILTER_TABLE|netlink.RT_FILTER_SRC,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rules)).To(Equal(0))

			return nil
		})
		Expect(err).NotTo(HaveOccurred())

		// DEL can be called multiple times, make sure no error is returned
		// if the device is already removed.
		err = originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})
})
