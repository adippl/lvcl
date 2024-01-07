/*
 *  lvcl is a simple program clustering libvirt servers
 *  Copyright (C) 2020 Adam Prycki (email: adam.prycki@gmail.com)
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License along
 *  with this program; if not, write to the Free Software Foundation, Inc.,
 *  51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
 */
package main

import "fmt"

const(
	NodeStateNil=iota
	NodeEvacuate
	NodeMaintenance
	NodeOffline
	NodeOnline
	NodeNotReady
	)

type Node struct{
	Hostname		string
	NodeAddress		string
	LibvirtAddress	string
	State 			int //{ NodeStateNil, NodeEvacuate }
	NodeStateString	string //{ NodeStateNil, NodeEvacuate }
	Weight			uint
	
	HwStats			[]Cluster_utilization
	Usage			[]Cluster_utilization `json:"-"`
	}

type NodeStats struct{
	totalCores	uint64
	totalMem	uint64
	
	cpuKernel	uint64
	cpuUser		uint64
	cpuIdle		uint64
	cpuIo		uint64
	
	memTotal	uint64
	memFree		uint64
	memBuffers	uint64
	memCached	uint64
	}

func (n Node)GetNodeStateString() string {
	switch n.State {
	case NodeStateNil:
		return "NodeStateNil"
	case NodeEvacuate:
		return "NodeEvacuate"
	case NodeMaintenance:
		return "NodeMaintenance"
	case NodeOffline:
		return "NodeOffline"
	case NodeOnline:
		return "NodeOnline"
	case NodeNotReady:
		return "NodeNotReady"
	default:
		return "Node.GetNodeStateString() ERROR"}}

func (n *Node)fixNodeLoadedFromConfig(){
	if n.State == NodeEvacuate || n.NodeStateString == "NodeEvacuate" {
		n.State = NodeEvacuate
		n.NodeStateString = "NodeEvacuate"
		return}
	if n.State == NodeMaintenance || n.NodeStateString == "NodeMaintenance" {
		n.State = NodeMaintenance
		n.NodeStateString = "NodeMaintenance"
		return}
	n.State = NodeStateNil
	n.NodeStateString = "NodeStateNil"
	return}

func (n *Node)fixNodeStateString(){
	n.NodeStateString = n.GetNodeStateString()}

func (p *Node)dump(){
	fmt.Printf("\n dumping Node %+v \nEND\n", *p)}

func (n Node)IsNodeReadyForResources() bool {
	switch n.State {
	case NodeOnline:
		return true
	default:
		return false
	}
}

func (n Node)CanSchedulerTouchResourceOnTheNode() bool {
	switch n.State {
	case NodeMaintenance:
		return false
	case NodeOffline:
		return false
	case NodeNotReady:
		return false
	default:
		return true
	}
}
