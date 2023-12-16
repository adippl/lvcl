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

import "gitlab.com/libvirt/libvirt-go-module"
import "fmt"
import "io/ioutil"
import "time"
import "os/exec"

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
	lvdVmStateStopped
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
	live_migration		bool
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
		live_migration:		false,
		daemonConneciton: conn,
		}
	l_lvd.getNodeInfo()
	go l_lvd.updateStats()
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
	fmt.Printf("== local libvirtd resources and domains ==\n");
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
		lg.err(fmt.Sprintf("startVM %s", v.Name), err)
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
	
	doms, err := l.daemonConneciton.ListAllDomains(0)
	if err != nil {
		lg.err("DEBUG l.daemonConneciton.ListAllDomains(0)", err)
		return}
	err=nil
	lg.msg_debug(3, fmt.Sprintf("l.daemonConneciton.ListAllDomains(0) returned %+v", doms))
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
				case libvirt.DOMAIN_SHUTOFF:
					l.domStates[name]=lvdVmStateStopped
				default:
					l.domStates[name]=lvdVmStateNil}
			//}else{
			//	lg.err("updateDomStates", err)}
//		}else{
//			lg.err("updateDomStates last GetName",err)}
		dom.Free()}}

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

func (l lvd)Get_running_resources() *[]Cluster_resource {
	var ret []Cluster_resource
	l.updateDomStates()
	
	for name,v := range l.domStates {
		dom := config.GetCluster_resourcebyName(&name)
		if dom == nil {
			continue}
		//if dom.State = resource_state_stopped {
		//	// don't send info about stopped domains
		//	continue)
		dom.Name = name
		switch v {
		case lvdVmStateStarting:
			dom.State = resource_state_starting
		case lvdVmStateRunning:
			dom.State = resource_state_running
		case lvdVmStateStopping:
			dom.State = resource_state_stopping
		case lvdVmStateStopped:
			dom.State = resource_state_stopped
		case lvdVmStatePaused:
			dom.State = resource_state_paused
		default:
			dom.State = resource_state_other
		}
		ret = append(ret,*dom)
		}
	return &ret
	}


func (l lvd)Get_utilization() *[]Cluster_utilization {
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

func (l lvd)Start_resource(name string) bool {
	vm := config.GetCluster_resourcebyName(&name)
	if(vm==nil){
		lg.msgERR("lvd.Start_resource() config.GetVMbyName returned null pointer")
		return false }
	lg.msg_debug(1,
		fmt.Sprintf("Node %s starts resource %s with %s controller",
			config._MyHostname(),
			name,
			vm.CtlString()))
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

func (l lvd)Stop_resource(name string) bool {
	var ret bool = false
	lg.msg_debug(5, fmt.Sprintf("lvd.Stop_resource() starts to stop %s resources", name))
	//dom := l.GetDomainPtr(name)
	//if(dom==nil){
	//	lg.msg(fmt.Sprintf("lvd.Stop_resource() GetDomainPtr returned %p", dom))
	//	dom.Free()
	//	return(false)}
	//state := lvd_state_to_lvcl_state(dom)
	//if state == resource_state_stopped {
	//	lg.msg(fmt.Sprintf("lvd.Stop_resource() undefines %s", name))
	//	if e := dom.Undefine() ; e != nil {
	//		lg.msgERR(fmt.Sprintf("lvd.stop_resource domain failed to undefine %s", name))
	//		ret=false
	//	}else{
	//		ret=true}}
	//if state == resource_state_running {
	//	lg.msg(fmt.Sprintf("lvd.Stop_resource() shutdown %s", name))
	//	if e := dom.Shutdown() ; e != nil {
	//		lg.err(fmt.Sprintf("lvd.stop_resource domain failed to stop %s", name), e)
	//		ret=false
	//	}else{
	//		ret=true}}
	//lg.msg(fmt.Sprintf("lvd.Stop_resource() GetDomainPtr returned %d", _stateString( state )))
	exec.Command("virsh", "shutdown", name).Run()
	exec.Command("virsh", "undefine", name).Run()
	//dom.Free()
	return(ret)}

func lvd_state_to_lvcl_state(dom *libvirt.Domain) int {
	state,_,_ := dom.GetState()
	switch state {
		case libvirt.DOMAIN_RUNNING:
			return resource_state_running
		case libvirt.DOMAIN_SHUTOFF:
			return resource_state_stopped
		default:
			return resource_state_other}}

func (l lvd)Nuke_resource(name string) bool {
	var ret bool = false
	//doms, err = l.daemonConneciton.ListAllDomains(0)
	vm := config.GetCluster_resourcebyName(&name)
	if(vm==nil){
		return(false)}
	dom := l.GetDomainPtr(vm.Name)
	if(dom==nil){
		return(false)}
	err := dom.Destroy()
	if(err!=nil){
		lg.err(fmt.Sprintf("lvd.stop_resource domain failed to stop %s", vm.Name) ,err)
		return(false)
	}else{
		ret=true}
	dom.Free()
	return(ret)}

func (l lvd)Kill_controller() bool {
	l.lvdKill=true
	time.Sleep(time.Millisecond * 1000)
	//defer conn.Close()
	rc,err := l.daemonConneciton.Close()
	if(err!=nil){
		lg.err("lvd.kill_controller .Close() failed",err)}
	lg.msg(fmt.Sprintf("DEBUG lvd.kill_controller returned %d",rc))
	return(true)}
	
func (l lvd)Clean_resource(name string) bool {
	lg.msg("WARNING lvd.clean_controller IS A PLACEHOLDER FOR REAL METHOD")
	return(true)}
	
func (l lvd)Migrate_resource(name string, dest_node string) bool {
	lg.msg("WARNING lvd.migrate_controller IS A PLACEHOLDER FOR REAL METHOD")
	return(true)}

//func (l *lvd)clean_resource(name string) bool {
//	l.nuke_resource(&name)

// placeholder
// TODO implement
func (c lvd)Get_controller_health() bool {
	var isalive bool = false
	var err error = nil
	
	//hostname,err := c.daemonConneciton.GetHostname() 
	//if err != nil {
	//	lg.err(" libvirt couldn't get a hostname", err)}
	//lg.msg(fmt.Sprintf("DEBUG libvirt hostname %+v", hostname))
	//err=nil
	
	isalive,err = c.daemonConneciton.IsAlive()
	if err != nil {
		lg.err(fmt.Sprintf("libvirt IsAlive returned error and bool %b", isalive ), err)}
	return isalive }


func (c lvd)Get_live_migration_support() bool {
	return c.live_migration}
