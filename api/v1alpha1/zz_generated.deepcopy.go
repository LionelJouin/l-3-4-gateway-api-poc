//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BfdSpec) DeepCopyInto(out *BfdSpec) {
	*out = *in
	if in.Switch != nil {
		in, out := &in.Switch, &out.Switch
		*out = new(bool)
		**out = **in
	}
	if in.Multiplier != nil {
		in, out := &in.Multiplier, &out.Multiplier
		*out = new(uint16)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BfdSpec.
func (in *BfdSpec) DeepCopy() *BfdSpec {
	if in == nil {
		return nil
	}
	out := new(BfdSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BgpAuth) DeepCopyInto(out *BgpAuth) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BgpAuth.
func (in *BgpAuth) DeepCopy() *BgpAuth {
	if in == nil {
		return nil
	}
	out := new(BgpAuth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BgpSpec) DeepCopyInto(out *BgpSpec) {
	*out = *in
	if in.RemoteASN != nil {
		in, out := &in.RemoteASN, &out.RemoteASN
		*out = new(uint32)
		**out = **in
	}
	if in.LocalASN != nil {
		in, out := &in.LocalASN, &out.LocalASN
		*out = new(uint32)
		**out = **in
	}
	in.BFD.DeepCopyInto(&out.BFD)
	if in.RemotePort != nil {
		in, out := &in.RemotePort, &out.RemotePort
		*out = new(uint16)
		**out = **in
	}
	if in.LocalPort != nil {
		in, out := &in.LocalPort, &out.LocalPort
		*out = new(uint16)
		**out = **in
	}
	if in.Auth != nil {
		in, out := &in.Auth, &out.Auth
		*out = new(BgpAuth)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BgpSpec.
func (in *BgpSpec) DeepCopy() *BgpSpec {
	if in == nil {
		return nil
	}
	out := new(BgpSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewayRouter) DeepCopyInto(out *GatewayRouter) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewayRouter.
func (in *GatewayRouter) DeepCopy() *GatewayRouter {
	if in == nil {
		return nil
	}
	out := new(GatewayRouter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GatewayRouter) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewayRouterList) DeepCopyInto(out *GatewayRouterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GatewayRouter, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewayRouterList.
func (in *GatewayRouterList) DeepCopy() *GatewayRouterList {
	if in == nil {
		return nil
	}
	out := new(GatewayRouterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GatewayRouterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewayRouterSpec) DeepCopyInto(out *GatewayRouterSpec) {
	*out = *in
	in.Bgp.DeepCopyInto(&out.Bgp)
	in.Static.DeepCopyInto(&out.Static)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewayRouterSpec.
func (in *GatewayRouterSpec) DeepCopy() *GatewayRouterSpec {
	if in == nil {
		return nil
	}
	out := new(GatewayRouterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GatewayRouterStatus) DeepCopyInto(out *GatewayRouterStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GatewayRouterStatus.
func (in *GatewayRouterStatus) DeepCopy() *GatewayRouterStatus {
	if in == nil {
		return nil
	}
	out := new(GatewayRouterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Network) DeepCopyInto(out *Network) {
	*out = *in
	if in.NetworkAttachementAnnotation != nil {
		in, out := &in.NetworkAttachementAnnotation, &out.NetworkAttachementAnnotation
		*out = new(NetworkAttachementAnnotation)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Network.
func (in *Network) DeepCopy() *Network {
	if in == nil {
		return nil
	}
	out := new(Network)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkAttachementAnnotation) DeepCopyInto(out *NetworkAttachementAnnotation) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkAttachementAnnotation.
func (in *NetworkAttachementAnnotation) DeepCopy() *NetworkAttachementAnnotation {
	if in == nil {
		return nil
	}
	out := new(NetworkAttachementAnnotation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StaticSpec) DeepCopyInto(out *StaticSpec) {
	*out = *in
	in.BFD.DeepCopyInto(&out.BFD)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StaticSpec.
func (in *StaticSpec) DeepCopy() *StaticSpec {
	if in == nil {
		return nil
	}
	out := new(StaticSpec)
	in.DeepCopyInto(out)
	return out
}