// Copyright 2024 SAP SE
// SPDX-License-Identifier: Apache-2.0

package networkdata

import "encoding/json"

type NetworkData struct {
	Links    []L2             `json:"links"  yaml:"links"`
	Networks []L3             `json:"networks"  yaml:"networks"`
	Services []NetworkService `json:"services"  yaml:"services"`
}

type L2 struct {
	EthernetMACAddress *string          `json:"ethernet_mac_address,omitempty"  yaml:"ethernet_mac_address,omitempty"`
	ID                 string           `json:"id"  yaml:"id"`
	MTU                *float64         `json:"mtu"  yaml:"mtu"`
	Type               InterfaceType    `json:"type"  yaml:"type"`
	VifID              *string          `json:"vif_id,omitempty"  yaml:"vif_id,omitempty"`
	BondLinks          []string         `json:"bond_links,omitempty"  yaml:"bond_links,omitempty"`
	BondMode           *PortBondingType `json:"bond_mode,omitempty"  yaml:"bond_mode,omitempty"`
	VLANID             *int64           `json:"vlan_id,omitempty"  yaml:"vlan_id,omitempty"`
	VLANLink           *string          `json:"vlan_link,omitempty"  yaml:"vlan_link,omitempty"`
	VLANMACAddress     *string          `json:"vlan_mac_address,omitempty"  yaml:"vlan_mac_address,omitempty"`
}

type L3 struct {
	ID        int                             `json:"id"  yaml:"id"`
	IPAddress *string                         `json:"ip_address,omitempty"  yaml:"ip_address,omitempty"`
	Link      string                          `json:"link"  yaml:"link"`
	Netmask   *string                         `json:"netmask,omitempty"  yaml:"netmask,omitempty"`
	NetworkID string                          `json:"network_id"  yaml:"network_id"`
	Routes    []L3IPVRoutingConfigurationItem `json:"routes,omitempty"  yaml:"routes,omitempty"`
	Type      L3IPVNetworkType                `json:"type"  yaml:"type"`
}

type L3IPVRoutingConfigurationItem struct {
	Gateway  string                 `json:"gateway"  yaml:"gateway"`
	Netmask  string                 `json:"netmask"  yaml:"netmask"`
	Network  string                 `json:"network"  yaml:"network"`
	Services []ServiceOnAIPVNetwork `json:"services,omitempty"  yaml:"services,omitempty"`
}

type ServiceOnAIPVNetwork struct {
	Address string      `json:"address"  yaml:"address"`
	Type    ServiceType `json:"type"  yaml:"type"`
}

type NetworkService struct {
	Address string      `json:"address"  yaml:"address"`
	Type    ServiceType `json:"type"  yaml:"type"`
}

type PortBondingType string

const (
	ActiveBackup PortBondingType = "active-backup"
	BalanceAlb   PortBondingType = "balance-alb"
	BalanceRr    PortBondingType = "balance-rr"
	BalanceTlb   PortBondingType = "balance-tlb"
	BalanceXor   PortBondingType = "balance-xor"
	Broadcast    PortBondingType = "broadcast"
	The8023Ad    PortBondingType = "802.3ad"
)

type InterfaceType string

const (
	Bond      InterfaceType = "bond"
	Bridge    InterfaceType = "bridge"
	Dvs       InterfaceType = "dvs"
	HwVeb     InterfaceType = "hw_veb"
	Hyperv    InterfaceType = "hyperv"
	Ovs       InterfaceType = "ovs"
	Phy       InterfaceType = "phy"
	Tap       InterfaceType = "tap"
	VLAN      InterfaceType = "vlan"
	Vhostuser InterfaceType = "vhostuser"
	Vif       InterfaceType = "vif"
)

type ServiceType string

const (
	DNS ServiceType = "dns"
)

type L3IPVNetworkType string

const (
	Ipv4      L3IPVNetworkType = "ipv4"
	Ipv4DHCP  L3IPVNetworkType = "ipv4_dhcp"
	Ipv6      L3IPVNetworkType = "ipv6"
	Ipv6DHCP  L3IPVNetworkType = "ipv6_dhcp"
	Ipv6Slaac L3IPVNetworkType = "ipv6_slaac"
)

func UnmarshalNetworkData(data []byte) (NetworkData, error) {
	var r NetworkData
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *NetworkData) Marshal() ([]byte, error) {
	return json.Marshal(r)
}
