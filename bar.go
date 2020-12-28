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
import "encoding/json"
//import "os"
import "io/ioutil"

func bar(){
	fmt.Println("bar")
	test := Conf{ UUID:"testuuid", 
		DomainDefinitionDir: "domains/",
		Nodes: []Node{
			Node{
				Hostname: "r210II-1",
				NodeAddress: "10.0.6.11:6798",
				LibvirtAddress: "10.0.6.11",
				NodeState: NodePreparing,
				Weight: 100},
			Node{
				Hostname: "r210II-2",
				NodeAddress: "10.0.6.12:6798",
				LibvirtAddress: "10.0.6.12",
				NodeState: NodePreparing,
				Weight: 100},
			Node{
				Hostname: "r210II-3",
				NodeAddress: "10.0.6.13:6798",
				LibvirtAddress: "10.0.6.13",
				NodeState: NodePreparing,
				Weight: 100}},
		VMs: []VM{
			VM{
				Name: "gh-test",
				DomainDefinition: "tests struct embedded in main cluster.conf",
				VCpus: 1,
				HwCpus: 0,
				VMem: 512,
				HwMem: 512,
				MigrationTimeout: 180,
				MigrateLive: true},
				},
		BalanceMode: Cpus,
		ResStickiness:50,
		GlobMigrationTimeout:120,
		GlobLiveMigrationBlock:false,
		Maintenance: true,
		VCpuMax: 8,
		HwCpuMax: 8,
		VMemMax: 8192,
		HwMemMax: 8192,
		HeartbeatInterval: 1000,
		ClusterTickInterval: 100,
		TCPport: "6798",
		UnixSocket: "lvcl.sock",
		LogLocal: "loc.log",
		LogCombined: "cmb.log",
		}
	
	fmt.Printf("%+v\n", test)
	//var x int = 5
	//confser, err := json.MarshalIndent(x,"","	")
	confser, err := json.MarshalIndent(test,"","	")
	//confser, err := json.Marshal(test)
	if err != nil {
		fmt.Println("Can't serislize", test)
		}
	fmt.Println(confser);
	
	ioutil.WriteFile("./cluster.json",confser,0644)
		

	}

	/* */
