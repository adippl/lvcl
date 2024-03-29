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
import "sync"
import "reflect"

const( confDir="./" )
const( confFile="cluster.json" )

type Conf struct {
	UUID string
	DomainDefinitionDir string
	Nodes []Node
	Quorum uint
	BalanceMode uint
	ResStickiness uint
	GlobMigrationTimeout uint
	GlobLiveMigrationBlock bool
	Maintenance bool
	Resources []Cluster_resource
	ResourceControllers map[string]bool
	rwmux sync.RWMutex `json:"-"`
	Epoch uint64 `json:"-"`
	numberOfNodes int `json:"-"`
	
	VCpuMax uint
	HwCpuMax uint
	VMemMax uint
	HwMemMax uint
	
	HeartbeatInterval uint
	ClusterTick uint
	HealthDeltaUpdateDelay uint
	//NodeHealthCheckInterval uint
	ReconnectLoopDelay uint
	HeartbeatTimeFormat string
	
	EnabledResourceControllers map[int]bool
	ClusterBalancerDelay int
	
	TCPport string
	UnixSocket string
	ConfFileHash string `json:"-"`
	ConfFileHashRaw []byte `json:"-"`
	MyHostname string `json:"-"`
	ConfHashCheck bool
	DefaultEventTimeoutTimeSec uint
	
	LogLocal string
	LogCombined string
	
	DaemonLogging	bool
	
	DebugLevel uint
	DebugNetwork bool
	DebugLogger bool
	DebugNoRemoteLogging bool
	DebugRawLogging bool
	DebugHeartbeat bool
	DebugLibvirtShowDomStates bool
	debugRunOnAnyHost			bool
	_debug_one_node_cluster bool `json:"-"`
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
	
	var h=sha256.New()
	if _,err:=io.Copy(h,file);err!=nil {
		fmt.Println("ERR hashing cluster.json")}
	config.ConfFileHash=fmt.Sprintf("%x",h.Sum(nil))
	config.ConfFileHashRaw=h.Sum(nil)
	file.Seek(0, io.SeekStart)
	fmt.Println("cluster.json sha256 "+config.ConfFileHash)
	
	fmt.Println("reading cluster.json")	
	raw, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("ERR reading cluster.json")
		os.Exit(10);}
		
	config.rwmux = sync.RWMutex{}
	//config.rwmux.Lock()
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
	
	json.Unmarshal(raw,&config)
	
	
	loadAllVMfiles()
	config._fix_resource_IDs()
	config._fix_node_states()
	config._fix_node_Resource_IDs()
	config._saveAllResources()
	
	//config.dumpConfig()
	//config.rwmux.Unlock()
	
	config.numberOfNodes = len(config.Nodes)
	
	if config.MyHostname == "x270" {
		config._debug_one_node_cluster = true
		config.Quorum = 1
		config.numberOfNodes++
		config.Nodes = append(config.Nodes,
			Node{
				Hostname: "x270",
				NodeAddress: "127.0.0.1:6798",
				LibvirtAddress: "127.0.0.1",
				NodeStateString: "",
				Weight: 1004,
				HwStats: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	20,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	8,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	18432,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	18432,
						},
					},
				})
		config._fix_node_Resource_IDs()
	}else{
		config._debug_one_node_cluster = false
		}
	}

func loadAllVMfiles(){
	f,e:=ioutil.ReadDir("domains")
	if e!=nil{
		fmt.Println(e)}
	for _, f := range f{
		VMReadFile("domains/"+f.Name())}}



func VMReadFile(path string) error {
	var res Cluster_resource
	var tmpres *Cluster_resource
	
	fmt.Println("reading "+path)
	file, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(11)}
	defer file.Close()
	fmt.Println("reading "+path )
	raw, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("ERR reading "+path )
		os.Exit(11);}
	
	json.Unmarshal(raw,&res)
	res.ConfFile = path

	tmpres=config.GetCluster_resourcebyName(&res.Name)
	if(tmpres != nil){
		fmt.Printf("err resource with name %s already exists\n", res.Name)
		return fmt.Errorf("err resource with name %s already exists", res.Name)}

	//make sure there are no duplicated libvirt resources
	if res.ResourceController_id == resource_controller_id_libvirt {
		xmlfile := res.Strs["DomainXML"]
		tmpres=config.GetVMbyDomain(&xmlfile)
		if(tmpres != nil){
			fmt.Printf("DEBUG err VM with domain file %s already exists\n",
				xmlfile)
			return fmt.Errorf("err VM with domain file %s already exists",
				xmlfile)}}
	
	config.rwmux.Lock()
	config.Resources = append(config.Resources,res)
	config.rwmux.Unlock()
	
	return nil}


func (c *Conf)dumpConfig(){
	fmt.Printf("\n\n\n\n LOADED CONFIG\n\n\n\n")
	raw, _ := json.MarshalIndent(&config,"","	")
	fmt.Println(string(raw))
	}


func (c *Conf)GetVMbyDomain(argDomain *string)(v *Cluster_resource){
	c.rwmux.RLock()
	for _,v:= range c.Resources{
		if	v.ResourceController_id == resource_controller_id_libvirt &&
			v.Strs["DomainXML"] == *argDomain{
			c.rwmux.RUnlock()
			return &v}}
	config.rwmux.RUnlock()
	return nil}

func (c *Conf)GetCluster_resourcebyName(argName *string)(v *Cluster_resource){
	c.rwmux.RLock()
	for _,t:= range c.Resources{
		if t.Name == *argName{
			c.rwmux.RUnlock()
			return &t}}
	config.rwmux.RUnlock()
	return nil}

func (c *Conf)GetCluster_resourcebyName_no_mutex(argName *string)(v *Cluster_resource){
	for k,_ := range c.Resources{
		if c.Resources[k].Name == *argName{
			return &c.Resources[k]}}
	return nil}


func (c *Conf)setResourceState(	
	name *string,
	ID int,
	state *string) bool {
	
	return false}


func (c *Conf)GetNodebyHostname(argHostname *string) *Node {
	for _,t:= range c.Nodes{
		if t.Hostname == *argHostname{
			return &t}}
	return nil}

func (c *Conf)CheckIfNodeExists(argHostname *string) bool {
	var ret bool
	ret=false
	c.rwmux.RLock()
	for _,t:= range c.Nodes{
		if t.Hostname == *argHostname{
			ret=true}}
	c.rwmux.RUnlock()
	return ret}

func writeExampleConfig(){
	fmt.Println("Creatimg example config for lvcl")
	testConfig := Conf{ UUID: "testuuid",
		DomainDefinitionDir: "domains/",
		Nodes: []Node{
			Node{
				Hostname: "ghn-s920-3",
				NodeAddress: "10.0.5.57:6798",
				LibvirtAddress: "10.0.5.57",
				NodeStateString: "",
				Weight: 1003,
				HwStats: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	4,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	2,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	3000,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	3807,
						},
					},
				},
			Node{
				Hostname: "ghn-s920-4",
				NodeAddress: "10.0.5.58:6798",
				LibvirtAddress: "10.0.5.58",
				NodeStateString: "",
				Weight: 1002,
				HwStats: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	4,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	2,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	3000,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	3807,
						},
					},
				},
			Node{
				Hostname: "ghn-s920-5",
				NodeAddress: "10.0.5.59:6798",
				LibvirtAddress: "10.0.5.59",
				NodeStateString: "",
				Weight: 1001,
				HwStats: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	4,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	2,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	3000,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	3807,
						},
					},
				}},
		ResourceControllers: map[string]bool{
			"libvirt": true},
		Resources: []Cluster_resource{
			Cluster_resource{
				ResourceController_name: "libvirt",
				ResourceController_id: resource_controller_id_libvirt,
				Name: "gh-test-1",
				Id: 1001,
				State: resource_state_running,
				State_name: "off",
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	0,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	0,
						},
					},
				Strs: map[string]string{
					"DomainXML": "/var/lib/libvirt/images/domains/gh-test-1.xml",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "libvirt",
				ResourceController_id: resource_controller_id_libvirt,
				Name: "gh-test-2",
				Id: 1002,
				State: resource_state_running,
				State_name: "off",
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	0,
						},
					},
				Strs: map[string]string{
					"DomainXML": "/var/lib/libvirt/images/domains/gh-test-2.xml",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "libvirt",
				ResourceController_id: resource_controller_id_libvirt,
				Name: "gh-test-3",
				Id: 1003,
				State: resource_state_running,
				State_name: "off",
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	0,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	512,
						},
					},
				Strs: map[string]string{
					"DomainXML": "/var/lib/libvirt/images/domains/gh-test-3.xml",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "libvirt",
				ResourceController_id: resource_controller_id_libvirt,
				Name: "gh-test-4",
				Id: 1004,
				State: resource_state_running,
				State_name: "off",
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	512,
						},
					},
				Strs: map[string]string{
					"DomainXML": "/var/lib/libvirt/images/domains/gh-test-4.xml",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "libvirt",
				ResourceController_id: resource_controller_id_libvirt,
				Name: "gh-test-5",
				Id: 1005,
				State: resource_state_running,
				State_name: "on",
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	512,
						},
					},
				Strs: map[string]string{
					"DomainXML": "/var/lib/libvirt/images/domains/gh-test-5.xml",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "libvirt",
				ResourceController_id: resource_controller_id_libvirt,
				Name: "gh-test-6",
				Id: 1006,
				State: resource_state_stopped,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	0,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	0,
						},
					},
				Strs: map[string]string{
					"DomainXML": "/var/lib/libvirt/images/domains/gh-test-6.xml",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "dummy",
				ResourceController_id: resource_controller_id_dummy,
				Name: "dummy-gh-test-7",
				Id: 1006,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						//Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						//Id:		utilization_hw_mem,
						Value:	512,
						},
					},
				Strs: map[string]string{
					"DomainXML": "tests struct embedded in main cluster.conf",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "dummy",
				ResourceController_id: resource_controller_id_dummy,
				Name: "basicResource 2 HIGH mem",
				Id: 11,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	1024,
						},
					},
				Strs: map[string]string{
					"DomainXML": "tests struct embedded in main cluster.conf",
				},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : true,
				},
			},
			Cluster_resource{
				ResourceController_name: "dummy",
				ResourceController_id: resource_controller_id_dummy,
				Name: "dummy resource 3 Too High mem",
				Id: 12,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	99999,
						},
					},
				Strs: map[string]string{
					"DomainXML": "tests struct embedded in main cluster.conf",
					},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : false,
					},
				},
			Cluster_resource{
				ResourceController_name: "dummy",
				ResourceController_id: resource_controller_id_dummy,
				Name: "dummy resource 4 too high cpu",
				Id: 13,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		onutilization_vpcus,
						Value:	100,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	1024,
						},
					},
				Strs: map[string]string{
					"DomainXML": "tests struct embedded in main cluster.conf",
					},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : false,
					},
				},
			Cluster_resource{
				ResourceController_name: "dummy",
				ResourceController_id: resource_controller_id_dummy,
				Name: "dummy resource 5 high cpu",
				Id: 14,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	3,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					},
				Strs: map[string]string{
					"DomainXML": "tests struct embedded in main cluster.conf",
					},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : false,
					},
				},
			Cluster_resource{
				ResourceController_name: "dummy",
				ResourceController_id: resource_controller_id_dummy,
				Name: "dummy resource 6 high cpu",
				Id: 15,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	3,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	512,
						},
					},
				Strs: map[string]string{
					"DomainXML": "tests struct embedded in main cluster.conf",
					},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : false,
					},
				},
			Cluster_resource{
				ResourceController_name: "dummy",
				ResourceController_id: resource_controller_id_dummy,
				Name: "dummy resource 7 high mem",
				Id: 16,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						//Id:		utilization_vpcus,
						Value:	2,
						},
					Cluster_utilization{
						Name:	"vMEM",
						//Id:		utilization_vmem,
						Value:	10240,
						},
					},
				Strs: map[string]string{
					"DomainXML": "tests struct embedded in main cluster.conf",
					},
				Ints: map[string]int{
					"MigrationTimeout" : 180,
					},
				Bools: map[string]bool{
					"MigrateLive" : false,
					},
				},
		},
		BalanceMode: Cpus,
		ResStickiness:50,
		GlobMigrationTimeout:120,
		GlobLiveMigrationBlock:false,
		Maintenance: false,
		VCpuMax: 8,
		HwCpuMax: 8,
		VMemMax: 8192,
		HwMemMax: 8192,
		ClusterBalancerDelay: 3,
		Quorum: 2,
		EnabledResourceControllers: map[int]bool{
			resource_controller_id_libvirt: true,
			resource_controller_id_dummy:  false,
			},
		HeartbeatInterval: 1000,
		ClusterTick: 2000,
		ConfHashCheck: true,
		HealthDeltaUpdateDelay: 250,
		//NodeHealthCheckInterval: 1000,
		ReconnectLoopDelay: 2500,
		HeartbeatTimeFormat: "2006-01-02 15:04:05",
		TCPport: "6798",
		UnixSocket: "./lvcl.sock",
		DefaultEventTimeoutTimeSec: 15,
		LogLocal: "loc.log",
		LogCombined: "cmb.log",
		DaemonLogging:	true,
		DebugLevel: 5,
		DebugNetwork: false,
		DebugLogger: false,
		DebugNoRemoteLogging: false,
		DebugRawLogging: false,
		DebugHeartbeat: false,
		DebugLibvirtShowDomStates: true,
		debugRunOnAnyHost: true,
		}
	
	confser, err := json.MarshalIndent(testConfig,"","	")
	if err != nil {
		fmt.Println("Can't serislize", testConfig)
		}
	ioutil.WriteFile("./cluster.json",confser,0644)}


func (c *Conf)ClusterTick_sleep(){
	time.Sleep( time.Duration(c.ClusterTick) * time.Millisecond )}

func (c *Conf)ClusterHeartbeat_sleep(){
	time.Sleep( time.Duration(c.HeartbeatInterval) * time.Millisecond )}

func (c *Conf)GetField_uint(field string) (uint) {
	c.rwmux.RLock()
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	c.rwmux.RUnlock()
	return uint(f.Uint()) }

func (c *Conf)SetField_uint(field string, value uint){
	c.rwmux.Lock()
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	f.SetUint(uint64(value))
	c.rwmux.Unlock()}

func (c *Conf)GetField_string(field string) (string) {
	c.rwmux.RLock()
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	c.rwmux.RUnlock()
	return string(f.String()) }

func (c *Conf)SetField_string(field string, value string){
	c.rwmux.Lock()
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	f.SetString(value)
	c.rwmux.Unlock()}

func (c *Conf)GetField_bool(field string) (bool) {
	c.rwmux.RLock()
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	c.rwmux.RUnlock()
	return bool(f.Bool()) }

func (c *Conf)SetField_bool(field string, value bool){
	c.rwmux.Lock()
	r := reflect.ValueOf(c)
	f := reflect.Indirect(r).FieldByName(field)
	f.SetBool(bool(value))
	c.rwmux.Unlock()}


func (c *Conf)SetNewResources(res *[]Cluster_resource){
	c.rwmux.Lock()
	c.Resources = *res
	c.rwmux.Unlock()}
	
func (c *Conf)IncEpoch() {
	c.rwmux.Lock()
	c.Epoch++
	c.rwmux.Unlock()}

func (c *Conf)GetEpoch() uint64 {
	var e uint64
	c.rwmux.RLock()
	e = c.Epoch
	c.rwmux.RUnlock()
	return e}

func (c *Conf)isTheirEpochBehind(i uint64) bool {
	var b bool
	c.rwmux.RLock()
	b = (i < c.Epoch)
	c.rwmux.RUnlock()
	return b}

func (c *Conf)isTheirEpochAhead(i uint64) bool {
	var b bool
	c.rwmux.RLock()
	b = (i > c.Epoch)
	c.rwmux.RUnlock()
	return b}

func (c *Conf)_saveAllResources(){
	for k,_:=range c.Resources {
		c.Resources[k].SaveToFile()}}

func (r *Cluster_resource)_fix_IDs() {
	switch r.ResourceController_name {
		case "libvirt":
			r.ResourceController_id = resource_controller_id_libvirt
		case "dummy":
			r.ResourceController_id = resource_controller_id_dummy
		default:
			fmt.Printf("ERROR Couldn't fix ID on resoutce %s %+v\n",
				r.Name, r)}
	if len(r.State_name) != 0 {
		switch r.State_name {
			case "on":
				r.State = resource_state_running
			case "off":
				r.State = resource_state_stopped
			default:
				r.State = resource_state_other }}
	switch r.State {
		case resource_state_running:
			r.State_name = "on"
		case resource_state_stopped:
			r.State_name = "off"
	}
	for k,_:=range r.Util {
		r.Util[k]._fix_util_IDs()}
	}

func (r *Cluster_utilization)_fix_util_IDs() {
	////fix resource_controller ID
	//switch r.ResourceController_name {
	//	case "libvirt":
	//		r.ResourceController_id = resource_controller_id_libvirt
	//	case "dummy":
	//		r.ResourceController_id = resource_controller_id_dummy
	//	default:
	//		fmt.Printf("ERROR Couldn't fix ID on Cluster_utilization %s %+v\n",
	//			r.Name, r)}
	//fix Cluster_utilization ID
	switch r.Name {
		case "vCPUs":
			r.Id = utilization_vpcus
		case "hwCPUs":
			r.Id = utilization_hw_cores
		case "vMEM":
			r.Id = utilization_vmem
		case "hwMEM":
			r.Id = utilization_hw_mem
		default:
			fmt.Printf("ERROR Couldn't fix ID on Cluster_utilization %s %+v\n",
				r.Name, r)}}


func (c *Conf)_fix_resource_IDs(){
	for k,_:=range c.Resources {
		c.Resources[k]._fix_IDs()}}


func (c *Conf)_fix_node_states(){
	for k,_:=range c.Nodes {
		c.Nodes[k].fixNodeLoadedFromConfig()}}


func (c *Conf)_fix_node_Resource_IDs(){
	for k,_:=range c.Nodes {
		for kk,_:=range c.Nodes[k].HwStats {
			c.Nodes[k].HwStats[kk]._fix_util_IDs()}}}


func (c *Conf)GetMaintenance() (bool){
	// TODO make this thread safe
	return config.Maintenance}

func (c *Conf)GetMyHostname() (string){
	return config.GetField_string("MyHostname")}

func (c *Conf)_MyHostname()(hostname string){
	return c.MyHostname }

func (c *Conf)IsCtrlEnabled(i int) (bool){
	c.rwmux.RLock()	
	ret := c.EnabledResourceControllers[i]
	c.rwmux.RUnlock()	
	return ret}

func (c *Conf)GetNodes() []Node {
	c.rwmux.RLock()
	nodeCopy := make([]Node, len(config.Nodes))
	copy( nodeCopy, config.Nodes)
	c.rwmux.RUnlock()
	return nodeCopy
	}

func (c *Conf)GetNodes_map() map[string]Node {
	var nodes map[string]Node = make(map[string]Node)
	c.rwmux.RLock()
	for k,_ := range config.Nodes {
		nodes[ config.Nodes[k].Hostname ] = config.Nodes[k] }
	c.rwmux.RUnlock()
	return nodes
	}


func (c *Conf)setNodeState( name *string, state int ) {
	c.rwmux.RLock()
	for k,_:=range c.Nodes {
		if c.Nodes[k].Hostname == *name {
			c.Nodes[k].State = state
			c.Nodes[k].fixNodeStateString()
		}
	}
	c.rwmux.RUnlock()
}

func (c *Conf)setNodeState_if_not_special( name *string, state int ) {
	c.rwmux.RLock()
	for k,_:=range c.Nodes {
		if c.Nodes[k].Hostname == *name && c.Nodes[k].State != NodeMaintenance && c.Nodes[k].State != NodeEvacuate {
			c.Nodes[k].State = state
			c.Nodes[k].fixNodeStateString()
		}
	}
	c.rwmux.RUnlock()
}
