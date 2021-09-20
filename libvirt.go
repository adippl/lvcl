/*  lvcl is a simple program clustering libvirt servers
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
	brainIN				chan<- message
	lvdIN				<-chan message
	domStates			map[string]uint
	domDesiredState		map[string]uint
	daemonConneciton	*libvirt.Connect
	nodeCPUStats		*libvirt.NodeCPUStats
	nodeMemStats		*libvirt.NodeMemoryStats
	nodeInfo			*libvirt.NodeInfo
	nodeStats			NodeStats
	}

type lvdVM struct {
	name	string
	state	uint
	}

var lv *lvd

func NewLVD(a_brainIN chan<- message, a_lvdIN <-chan message) *lvd {
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
		brainIN:	a_brainIN,
		lvdIN:		a_lvdIN,
		domStates:	make(map[string]uint),
		nodeCPUStats:		nil,
		nodeMemStats:		nil,
		nodeInfo:			nil,
		domDesiredState:	make(map[string]uint),
		daemonConneciton: conn,
		}
	l_lvd.getNodeInfo()
	go l_lvd.updateStats()
	go l_lvd.messageHandler()
	go l_lvd.sendStatsToMaster()
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
	

func (l *lvd)startVM(v *VM) int {
	file, err := ioutil.ReadFile(v.DomainDefinition)
	if err != nil {
		lg.err("startVM", err)
		return 1}
	xml := string(file)
	
	// start modes TODO later
	// libvirt.DOMAIN_NONE
	// libvirt.DOMAIN_START_VALIDATE
	err = nil
	dom,err := l.daemonConneciton.DomainCreateXML(xml, libvirt.DOMAIN_NONE)
	if err != nil {
		lg.err("startVM", err)
		return 1}
	dom.Free()
	return 0}
	
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

func (l *lvd)sendStatsToMaster(){
	for {
		if(l.lvdKill){
			return }
		time.Sleep(time.Millisecond * time.Duration(config.ClusterTickInterval))
		
		l.updateDomStates()
		
		m := message{
			SrcHost:	config.MyHostname,
			DestHost:	"__master__",
			SrcMod:		msgModBrainController,
			DestMod:	msgModBrain,
			Time:		time.Now(),
			RpcFunc:	brainRpcSendingStats,
			Argc:		1,
			Argv:		[]string{"NodeStats in custom1",},
			custom1:	l.nodeStats,
			custom2:	l.domStates,
			}
		l.brainIN <- m }}

func (l *lvd)messageHandler(){
	var m message
	for {
		m = <-l.lvdIN
		fmt.Println("dummy message handle for lvd ", m)}}
