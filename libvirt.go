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

//import libvirt "gitlab.com/libvirt/libvirt-go@v7.0.0"
//https://gitlab.com/libvirt/libvirt-go.git
import libvirt "gitlab.com/libvirt/libvirt-go"
import "fmt"
import "io/ioutil"
import "time"

const(
	lvdVmStateNil=iota
	lvdVmStateBlocked
	lvdVmStateCrashed
	lvdVmStateFsHalt
	lvdVmStateFsResume
	lvdVmStateOther
	lvdVmStatePaused
	lvdVmStateRunning
	lvdVmStateShutdown
	lvdVmStateStarting
	lvdVmStateStopping
	lvdVmState
	
	lvdCpuKernel
	lvdCpuUser
	lvdCpuIdle
	lvdCpuIo
	)

type lvd struct {
	lvdKill				bool
	domStates			map[string]uint
	daemonConneciton	*libvirt.Connect
	nodeCPUStats		*libvirt.NodeCPUStats
	nodeMemStats		*libvirt.NodeMemoryStats
	nodeInfo			*libvirt.NodeInfo
	nodeStats			NodeStats
	utilization			*[]Cluster_utilization
	}

type lvdVM struct {
	name	string
	state	uint
	}

var lv *lvd

func NewLVD() *lvd {
	var err error
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		lg.err("libvirt NewConnect Local",err)
		defer conn.Close()
		return nil
		}
	// TODO close this connection at some point
	// defer close doesn't defer and closes connection immediately
	//defer conn.Close()
	
	l_lvd := lvd{
		lvdKill:	false,
		domStates:	make(map[string]uint),
		nodeCPUStats:		nil,
		nodeMemStats:		nil,
		nodeInfo:			nil,
		daemonConneciton: conn,
		}
	l_lvd.getNodeInfo()
	go l_lvd.updateStats()
	//go l_lvd.messageHandler()
	//go l_lvd.sendStatsToMaster()
	return &l_lvd }

func (l *lvd)getNodeInfo(){
	nodeInfo,err := l.daemonConneciton.GetNodeInfo()
	if err != nil {
		lg.err("libvirt.GetNodeInfo local",err)}
	l.nodeInfo = nodeInfo}

func (l *lvd)updateStats(){
	for{
		if l.lvdKill == true {
			return }
		cpustats, err := l.daemonConneciton.GetCPUStats(int(libvirt.NODE_CPU_STATS_ALL_CPUS), 0)
		if err != nil {
			lg.err("libvirt GetCPUStats",err)
			l.nodeCPUStats = nil
		}else{
			if l.nodeCPUStats != nil {
				// divide by / 1000000 to convert nanoseconds to milliseconds
				// 1000ms == 1s of cpu time
				// values above 1000 mean that system has multicore cpu
				l.nodeStats.cpuKernel=uint64((cpustats.Kernel - l.nodeCPUStats.Kernel) / 1000000)
				l.nodeStats.cpuUser=uint64((cpustats.User - l.nodeCPUStats.User) / 1000000)
				l.nodeStats.cpuIdle=uint64((cpustats.Idle - l.nodeCPUStats.Idle) / 1000000)
				l.nodeStats.cpuIo=uint64((cpustats.Iowait - l.nodeCPUStats.Iowait) / 1000000)
				}
			l.nodeCPUStats = cpustats }
		err = nil
		memstats, err := l.daemonConneciton.GetMemoryStats(int(libvirt.NODE_MEMORY_STATS_ALL_CELLS), 0)
		if err != nil {
			lg.err("libvirt GetMemoryStats",err)
			l.nodeMemStats = nil
		}else{
			l.nodeStats.memTotal=memstats.Total
			l.nodeStats.memFree=memstats.Free
			l.nodeStats.memBuffers=memstats.Buffers
			l.nodeStats.memCached=memstats.Cached
			l.nodeMemStats = memstats }
		//always sleep one second
		//constatnt time for cpu usage calcualtion
		time.Sleep(time.Second)}}

func (l *lvd)listDomains(){
	if l == nil {
		fmt.Println("lvd object ptr == nil")
		return }
	//fmt.Printf("NodeCPUStats %+v\n",l.nodeCPUStats)
	fmt.Printf("== local resources and domains ==\n");
	fmt.Printf("Host total cores=%d mem=%d", l.nodeStats.totalCores, l.nodeStats.totalMem/(1<<20))
	fmt.Printf("CPU: kernel=%d user=%d io=%d idle=%d\n",
		l.nodeStats.cpuKernel/10,
		l.nodeStats.cpuUser/10,
		l.nodeStats.cpuIo/10,
		l.nodeStats.cpuIdle/10)
	fmt.Printf("Memory: total=%dGiB free=%dMiB cache=%dMiB\n",
		l.nodeStats.memTotal/(1<<20),
		l.nodeStats.memFree/(1<<10),
		l.nodeStats.memCached/(1<<10))
	
	doms, err := l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
	    lg.err("libvirt listAllDomains",err)
		return}
	fmt.Printf("%d running domains:\n", len(doms))
	for _, dom := range doms {
		name, err := dom.GetName()
		if err == nil {
			fmt.Printf("  %s\n", name)}
		dom.Free()}
	fmt.Println(l.domStates)
	fmt.Printf("===================\n")}
	

func (l *lvd)startVM(v *Cluster_resource) bool {
	if v.ResourceController_id != resource_controller_id_libvirt {
		lg.msgERR("lvd.startVM() received Cluster_resource of incorrect type")
		return false}
	file, err := ioutil.ReadFile(v.Strs["DomainXML"])
	if err != nil {
		lg.err("startVM", err)
		return false}
	xml := string(file)
	
	// start modes TODO later
	// libvirt.DOMAIN_NONE
	// libvirt.DOMAIN_START_VALIDATE
	err = nil
	dom,err := l.daemonConneciton.DomainCreateXML(xml, libvirt.DOMAIN_NONE)
	if err != nil {
		lg.err("startVM failed", err)
		return false}
	dom.Free()
	return true}
	
func (l *lvd)updateDomStates(){
	l.domStates = make(map[string]uint)
	if l == nil {
		fmt.Println("lvd object ptr == nil")
		return }
	
//	doms, err := l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_RUNNING)
//	if err != nil {
//		return}
//	for _, dom := range doms {
//		name, err := dom.GetName()
//		if err == nil {
//			l.domStates[name] = lvdVmStateRunning }
//		dom.Free()}
	
//	doms, err = l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_PAUSED)
//	if err != nil {
//		return}
//	for _, dom := range doms {
//		name, err := dom.GetName()
//		if err == nil {
//			l.domStates[name] = lvdVmStatePaused}
//		dom.Free() }

	doms, err := l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_OTHER)
	if err != nil {
		return}
	for _, dom := range doms {
		name, err := dom.GetName()
		if(err == nil){
			l.domStates[name] = lvdVmStateOther
			//state,_,err := dom.GetState()
			_,_,err := dom.GetState()
			if(err != nil){
				l.domStates[name] = lvdVmStateOther
			}else{
				lg.err("updateDomStates", err)}}
		dom.Free()}
	
	doms, err = l.daemonConneciton.ListAllDomains(0)
	if err != nil {
		return}
// don't check for errors,
// something is wrong, false positives
// check later
	for _, dom := range doms {
//		name, err := dom.GetName()
		name, _ := dom.GetName()
		err=nil
//		if err != nil {
			l.domStates[name] = lvdVmStateOther
//			state,_,err := dom.GetState()
			state,_,_ := dom.GetState()
//			if(err != nil){
				switch state {
				case libvirt.DOMAIN_NOSTATE:
					l.domStates[name]=lvdVmStateNil
				case libvirt.DOMAIN_RUNNING:
					l.domStates[name]=lvdVmStateRunning
				case libvirt.DOMAIN_BLOCKED:
					l.domStates[name]=lvdVmStateBlocked
				case libvirt.DOMAIN_PAUSED:
					l.domStates[name]=lvdVmStatePaused
				case libvirt.DOMAIN_SHUTDOWN:
					l.domStates[name]=lvdVmStateShutdown
				case libvirt.DOMAIN_CRASHED:
					l.domStates[name]=lvdVmStateCrashed
				default:
					l.domStates[name]=lvdVmStateNil}
			//}else{
			//	lg.err("updateDomStates", err)}
//		}else{
//			lg.err("updateDomStates last GetName",err)}
		dom.Free()}}

//func (l *lvd)UpdataAndPrintDomStates(){
//	l.domStates = make(map[string]uint)
//	if l == nil {
//		fmt.Println("lvd object ptr == nil")
//		return }
//	doms, err := l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_RUNNING)
//	if err != nil {
//	    lg.err("libvirt listAllDomains",err)
//		return}
//	
//	fmt.Printf("%d running domains:\n", len(doms))
//	for _, dom := range doms {
//		name, err := dom.GetName()
//		if err == nil {
//			l.domStates[name] = lvdVmStateRunning
//			if config.DebugLibvirtShowDomStates {
//				fmt.Printf("running  %s\n", name) }}
//		dom.Free() }
//	
//	doms, err = l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_PAUSED)
//	if err != nil {
//	    lg.err("libvirt listAllDomains",err)
//		return}
//	
//	fmt.Printf("%d paused domains:\n", len(doms))
//	for _, dom := range doms {
//		name, err := dom.GetName()
//		if err == nil {
//			l.domStates[name] = lvdVmStatePaused
//			fmt.Printf("paused  %s\n", name) }
//		dom.Free() }
//
//	doms, err = l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_OTHER)
//	if err != nil {
//	    lg.err("libvirt listAllDomains",err)
//		return}
//	
//	fmt.Printf("%d other domains:\n", len(doms))
//	for _, dom := range doms {
//		name, err := dom.GetName()
//		if(err == nil){
//			l.domStates[name] = lvdVmStateOther
//			state,_,err := dom.GetState()
//			if(err != nil){
//				fmt.Printf("state:%+v  %s\n",state , name ) 
//			}else{
//				lg.err("updateDomStates", err)
//				fmt.Printf("state: %v %v\n",state , name ) }} 
//		dom.Free() }}

func (l *lvd)lvd_cluster_resource_template() *Cluster_resource {
	cr := Cluster_resource{
		ResourceController_name: "libvirt",
		ResourceController_id: resource_controller_id_libvirt,
		}
	return &cr
	}

func (l *lvd)lvd_cluster_utilization_template() *Cluster_utilization {
	cr := Cluster_utilization{
		ResourceController_name: "libvirt",
		ResourceController_id: resource_controller_id_libvirt,
		}
	return &cr
	}

func (l *lvd)Get_running_resources() *[]Cluster_resource {
	var ret []Cluster_resource
	
	for k,v := range l.domStates {
		dom := l.lvd_cluster_resource_template()
		dom.Name = k
		switch v {
		case lvdVmStateStarting:
			dom.State = resource_state_starting
		case lvdVmStateRunning:
			dom.State = resource_state_running
		case lvdVmStateStopping:
			dom.State = resource_state_stopping
		case lvdVmStatePaused:
			dom.State = resource_state_paused
		default:
			dom.State = resource_state_other
		}
		ret = append(ret,*dom)
		}
	return &ret
	}


func (l *lvd)Get_utilization() *[]Cluster_utilization {
	//var ret []utilization
	ret := make([]Cluster_utilization,0,10)
	var util *Cluster_utilization
	
	// cpu stats
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_cpu_time_kernel
	util.Value=l.nodeStats.cpuKernel
	ret = append(ret,*util)
	
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_cpu_time_user
	util.Value=l.nodeStats.cpuUser
	ret = append(ret,*util)
	
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_cpu_time_io
	util.Value=l.nodeStats.cpuIo
	ret = append(ret,*util)
	
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_cpu_time_idle
	util.Value=l.nodeStats.cpuIdle
	ret = append(ret,*util)
	
	// memory
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_mem_total_kb
	util.Value=l.nodeStats.memTotal
	ret = append(ret,*util)
	
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_mem_free_kb
	util.Value=l.nodeStats.memFree
	ret = append(ret,*util)
	
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_mem_cached_kb
	util.Value=l.nodeStats.memCached
	ret = append(ret,*util)
	
	util = l.lvd_cluster_utilization_template()
	util.Id=utilization_mem_buffers_kb
	util.Value=l.nodeStats.memBuffers
	ret = append(ret,*util)
	
	return &ret
	}

func (l *lvd)Start_resource(name string) bool {
	vm := config.GetCluster_resourcebyName(&name)
	if(vm==nil){
		lg.msg("config.GetVMbyName returned null pointer")
		return false }
	//if(l.startVM(vm)!=0){
	//	return(false)
	//}else{
	//	return(true)}}
	return l.startVM(vm)}

// remember to dom.Free() after you're done using Domain pointer
func (l *lvd)GetDomainPtr(domain_name string) *libvirt.Domain {
	var r_dom *libvirt.Domain = nil
	doms, err := l.daemonConneciton.ListAllDomains(0)
	if(err!=nil){
		return nil}
	for _, dom := range doms {
		dom_name, _ := dom.GetName()
		if(dom_name==domain_name){
			r_dom=&dom
		}else{
			// free all domain objects except for returned one
			dom.Free()}
		err=nil}
	return(r_dom)}

func (l *lvd)Stop_resource(name string) bool {
	var ret bool = false
	//doms, err = l.daemonConneciton.ListAllDomains(0)
	vm := config.GetCluster_resourcebyName(&name)
	if(vm==nil){
		return(false)}
	dom := l.GetDomainPtr(vm.Name)
	if(dom==nil){
		return(false)}
	err := dom.Shutdown()
	if(err!=nil){
		lg.err("lvd.stop_resource domain failed to stop",err)
		return(false)
	}else{
		ret=true}
	dom.Free()
	return(ret)}

func (l *lvd)Nuke_resource(name string) bool {
	var ret bool = false
	//doms, err = l.daemonConneciton.ListAllDomains(0)
	vm := config.GetCluster_resourcebyName(&name)
	if(vm==nil){
		return(false)}
	dom := l.GetDomainPtr(vm.Name)
	if(dom==nil){
		return(false)}
	err := dom.Shutdown()
	if(err!=nil){
		lg.err("lvd.stop_resource domain failed to stop",err)
		return(false)
	}else{
		ret=true}
	dom.Free()
	return(ret)}

func (l *lvd)Kill_controller() bool {
	l.lvdKill=true
	time.Sleep(time.Millisecond * 1000)
	//defer conn.Close()
	rc,err := l.daemonConneciton.Close()
	if(err!=nil){
		lg.err("lvd.kill_controller .Close() failed",err)}
	lg.msg(fmt.Sprintf("DEBUG lvd.kill_controller returned %d",rc))
	return(true)}
	
func (l *lvd)Clean_resource(name string) bool {
	lg.msg("WARNING lvd.clean_controller IS A PLACEHOLDER FOR REAL METHOD")
	return(true)}
	
func (l *lvd)Migrate_resource(name string, dest_node string) bool {
	lg.msg("WARNING lvd.migrate_controller IS A PLACEHOLDER FOR REAL METHOD")
	return(true)}

//func (l *lvd)clean_resource(name string) bool {
//	l.nuke_resource(&name)
