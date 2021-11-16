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
import "os"
import "encoding/json"
import "io/ioutil"
import "crypto/sha256"
import "io"
import "time"

const( confDir="./" )
const( confFile="cluster.json" )

type Conf struct {
	UUID string
	DomainDefinitionDir string
	Nodes []Node
	VMs []VM
	Quorum uint
	BalanceMode uint
	ResStickiness uint
	GlobMigrationTimeout uint
	GlobLiveMigrationBlock bool
	Maintenance bool
	Resources []cluster_resource
	ResourceControllers map[string]bool
	
	VCpuMax uint
	HwCpuMax uint
	VMemMax uint
	HwMemMax uint
	
	HeartbeatInterval uint
	ClusterTick uint
	ClusterTickInterval uint
	NodeHealthCheckInterval uint
	ReconnectLoopDelay uint
	HeartbeatTimeFormat string
	
	enabledResourceControllers map[uint]bool
	
	TCPport string
	UnixSocket string
	ConfFileHash string `json:"-"`
	ConfFileHashRaw []byte `json:"-"`
	MyHostname string `json:"-"`
	
	LogLocal string
	LogCombined string
	
	
	DebugLevel int
	DebugNetwork bool
	DebugLogger bool
	DebugNoRemoteLogging bool
	DebugRawLogging bool
	DebugHeartbeat bool
	DebugLibvirtShowDomStates bool
	}

var config Conf


func confPrint(p *Conf){
	fmt.Printf("%+v \n", *p)
	}

func confLoad(){
	file, err := os.Open("cluster.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(10)}
	defer file.Close()
	fmt.Println("reading cluster.json")	
	raw, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("ERR reading cluster.json")
		os.Exit(10);}
		
	config.MyHostname,err=os.Hostname()
	if(err != nil){
		panic(err)}
	
	/* TODO DEBUG 
	 * this codes tests if current hosts is in config file. 
	 * exits program if host is not in config
	if(config.getNodebyHostname(&config.MyHostname) == nil){
		fmt.Println("DEBUG err controller is started on hostname %s which doesn't exists in cluster configuration file\n",vm.DomainDefinition)
		os.Exit()}
	 */
	
	var h=sha256.New()
	if _,err:=io.Copy(h,file);err!=nil {
		fmt.Println("ERR hashing cluster.json")}
	config.ConfFileHash=fmt.Sprintf("%x",h.Sum(nil))
	config.ConfFileHashRaw=h.Sum(nil)
		
	json.Unmarshal(raw,&config)
	
	loadAllVMfiles()
	
	fmt.Printf("\n\n\n\n LOADED CONFIG\n\n\n\n")
	//dumpConfig()
	}

func loadAllVMfiles(){
	f,e:=ioutil.ReadDir("domains")
	if e!=nil{
		fmt.Println(e)}
	for _, f := range f{
		VMReadFile("domains/"+f.Name())}}

func (c *Conf)dumpConfig(){
	raw, _ := json.MarshalIndent(&config,"","	")
	fmt.Println(string(raw))
	}

func (c *Conf)GetVMbyName(argName *string)(v *VM){
	for _,t:= range c.VMs{
		if t.Name == *argName{
			return &t}}
	return nil}

func (c *Conf)GetVMbyDomain(argDomain *string)(v *VM){
	for _,t:= range c.VMs{
		if t.DomainDefinition == *argDomain{
			return &t}}
	return nil}


func (c *Conf)GetNodebyHostname(argHostname *string) *Node {
	for _,t:= range c.Nodes{
		if t.Hostname == *argHostname{
			return &t}}
	return nil}

func (c *Conf)CheckIfNodeExists(argHostname *string) bool {
	for _,t:= range c.Nodes{
		if t.Hostname == *argHostname{
			return true}}
	return false}

func writeExampleConfig(){
	fmt.Println("Creatimg example config for lvcl")
	testConfig := Conf{ UUID: "testuuid",
		DomainDefinitionDir: "domains/",
		Nodes: []Node{
			Node{
				Hostname: "r210II-1",
				NodeAddress: "10.0.6.14:6798",
				LibvirtAddress: "10.0.6.14",
				NodeState: NodePreparing,
				Weight: 1003},
			Node{
				Hostname: "r210II-2",
				NodeAddress: "10.0.6.15:6798",
				LibvirtAddress: "10.0.6.15",
				NodeState: NodePreparing,
				Weight: 1002},
			Node{
				Hostname: "r210II-3",
				NodeAddress: "10.0.6.16:6798",
				LibvirtAddress: "10.0.6.16",
				NodeState: NodePreparing,
				Weight: 1001}},
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
		ResourceControllers: map[string]bool{
			"libvirt": true},
		Resources: []cluster_resource{
			cluster_resource{
				resourceController_name: "libvirt",
				resourceController_id: 0,
				id: 0,
				resource: VM{
					Name: "gh-test",
					DomainDefinition: "tests struct embedded in main cluster.conf",
					VCpus: 1,
					HwCpus: 0,
					VMem: 512,
					HwMem: 512,
					MigrationTimeout: 180,
					MigrateLive: true,
				},
			},
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
		Quorum: 2,
		enabledResourceControllers: map[uint]bool{
			resource_controller_id_libvirt: true},
		HeartbeatInterval: 1000,
		ClusterTick: 1000,
		ClusterTickInterval: 250,
		NodeHealthCheckInterval: 1000,
		ReconnectLoopDelay: 1000,
		HeartbeatTimeFormat: "2006-01-02 15:04:05",
		TCPport: "6798",
		UnixSocket: "lvcl.sock",
		LogLocal: "loc.log",
		LogCombined: "cmb.log",
		DebugLevel: 1,
		DebugNetwork: false,
		DebugLogger: false,
		DebugNoRemoteLogging: false,
		DebugRawLogging: false,
		DebugHeartbeat: false,
		DebugLibvirtShowDomStates: true,
		}
	
	confser, err := json.MarshalIndent(testConfig,"","	")
	if err != nil {
		fmt.Println("Can't serislize", testConfig)
		}
	ioutil.WriteFile("./cluster.json",confser,0644)}


func (c *Conf)_MyHostname()(hostname string){
	return c.MyHostname }

func (c *Conf)ClusterTick_sleep(){
	time.Sleep( time.Duration(c.ClusterTick) * time.Millisecond )}
