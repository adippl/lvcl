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
	NodeOffline=iota
	NodePreparing
	NodeOnline
	NodeEvacuate
	)

type Node struct{
	Hostname		string
	NodeAddress		string
	LibvirtAddress	string
	NodeState uint //{ NodeOffline, NodePreparing, NodeOnline, NodeEvacuate }
	Weight	uint
	
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


func (p *Node)dump(){
	fmt.Printf("\n dumping Node %+v \nEND\n", *p)}
