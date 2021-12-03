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
	Epoch int `json:"-"`
	
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
	
	EnabledResourceControllers map[uint]bool
	
	TCPport string
	UnixSocket string
	ConfFileHash string `json:"-"`
	ConfFileHashRaw []byte `json:"-"`
	MyHostname string `json:"-"`
	ConfHashCheck bool
	
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
	
	
	config.dumpConfig()
	//config.rwmux.Unlock()
	}

func loadAllVMfiles(){
	f,e:=ioutil.ReadDir("domains")
	if e!=nil{
		fmt.Println(e)}
	for _, f := range f{
		VMReadFile("domains/"+f.Name())}}



func VMReadFile(path string) error {
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
	
	var res Cluster_resource
	var tmpres *Cluster_resource
	json.Unmarshal(raw,&res)

	tmpres=config.GetCluster_resourcebyName(&res.Name)
	if(tmpres != nil){
		fmt.Printf("err resource with name %s already exists\n", res.Name)
		return fmt.Errorf("err resource with name %s already exists", res.Name)}

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

func (c *Conf)GetCluster_resourcebyName_RW(argName *string)(v *Cluster_resource){
	c.rwmux.RLock()
	for v,t:= range c.Resources{
		if t.Name == *argName{
			c.rwmux.RUnlock()
			return &c.Resources[v]}}
	config.rwmux.RUnlock()
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
				Weight: 1003,
				HwStats: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						Id:		utilization_vpcus,
						Value:	80,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						Id:		utilization_hw_cores,
						Value:	8,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
						Value:	18432,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						Id:		utilization_hw_mem,
						Value:	18432,
						},
					},
				},
			Node{
				Hostname: "r210II-2",
				NodeAddress: "10.0.6.15:6798",
				LibvirtAddress: "10.0.6.15",
				NodeState: NodePreparing,
				Weight: 1002,
				HwStats: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						Id:		utilization_vpcus,
						Value:	80,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						Id:		utilization_hw_cores,
						Value:	8,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
						Value:	10240,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						Id:		utilization_hw_mem,
						Value:	10240,
						},
					},
				},
			Node{
				Hostname: "r210II-3",
				NodeAddress: "10.0.6.16:6798",
				LibvirtAddress: "10.0.6.16",
				NodeState: NodePreparing,
				Weight: 1001,
				HwStats: []Cluster_utilization{
					Cluster_utilization{
						Name:	"vCPUs",
						Id:		utilization_vpcus,
						Value:	80,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						Id:		utilization_hw_cores,
						Value:	8,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
						Value:	6144,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						Id:		utilization_hw_mem,
						Value:	6144,
						},
					},
				}},
		ResourceControllers: map[string]bool{
			"libvirt": true},
		Resources: []Cluster_resource{
			Cluster_resource{
				ResourceController_name: "libvirt",
				ResourceController_id: resource_controller_id_libvirt,
				Name: "basicResource 1",
				Id: 10,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"VCPUs",
						Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"hwCPUs",
						Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
						Value:	1024,
						},
					Cluster_utilization{
						Name:	"hwMEM",
						Id:		utilization_hw_mem,
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
						Name:	"VCPUs",
						Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
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
				Name: "dummy resource 3 Very High mem",
				Id: 12,
				State: resource_state_running,
				Util: []Cluster_utilization{
					Cluster_utilization{
						Name:	"VCPUs",
						Id:		utilization_vpcus,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
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
						Id:		utilization_hw_cores,
						Value:	1,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
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
						Name:	"VCPUs",
						Id:		utilization_vpcus,
						Value:	3,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
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
						Name:	"VCPUs",
						Id:		utilization_vpcus,
						Value:	3,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
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
						Name:	"VCPUs",
						Id:		utilization_vpcus,
						Value:	2,
						},
					Cluster_utilization{
						Name:	"vMEM",
						Id:		utilization_vmem,
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
		Maintenance: true,
		VCpuMax: 8,
		HwCpuMax: 8,
		VMemMax: 8192,
		HwMemMax: 8192,
		Quorum: 2,
		EnabledResourceControllers: map[uint]bool{
			resource_controller_id_libvirt: true,
			resource_controller_id_dummy:  true,
			},
		HeartbeatInterval: 1000,
		ClusterTick: 1000,
		ConfHashCheck: true,
		ClusterTickInterval: 250,
		NodeHealthCheckInterval: 1000,
		ReconnectLoopDelay: 2500,
		HeartbeatTimeFormat: "2006-01-02 15:04:05",
		TCPport: "6798",
		UnixSocket: "./lvcl.sock",
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


func (c *Conf)_MyHostname()(hostname string){
	return c.MyHostname }

func (c *Conf)ClusterTick_sleep(){
	time.Sleep( time.Duration(c.ClusterTick) * time.Millisecond )}

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


//func (b *Brain)is_this_node_a_master() bool {
//func (b *Brain)getMasterNodeName() *string {

func (c *Conf)SetNewResourceState(res *[]Cluster_resource){
	c.rwmux.Lock()
	c.Resources = *res
	c.rwmux.Unlock()}
	
func (c *Conf)IncEpoch() {
	c.rwmux.Lock()
	c.Epoch++
	c.rwmux.Unlock()}

func (c *Conf)GetEpoch() int {
	var e int
	c.rwmux.RLock()
	e = c.Epoch
	c.rwmux.RUnlock()
	return e}

func (c *Conf)isTheirEpochBehind(i int) bool {
	var b bool
	c.rwmux.RLock()
	b = (i > c.Epoch)
	c.rwmux.RUnlock()
	return b}

func (c *Conf)isTheirEpochAhead(i int) bool {
	var b bool
	c.rwmux.RLock()
	b = (i < c.Epoch)
	c.rwmux.RUnlock()
	return b}

