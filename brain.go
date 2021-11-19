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

import "fmt"
import "time"
import "sync"

const(
	CLUSTER_TICK = time.Millisecond * 1000 
	INTERNAL_CLUSTER_TICK = time.Millisecond * 250 
	//config.ClusterTick = config.ClusterTick
	//INTERNAL_config.ClusterTick = config.ClusterTickInterval 
	)

const (
	HealthGreen=1
	HealthOrange=2
	HealthRed=5
	)

type Brain struct{
	isMaster			bool
	masterNode			*string
	
	killBrain			bool
	nominatedBy			map[string]bool
	voteCounterExists	bool
	quorum				uint
	
	ex_brn					<-chan message
	brn_ex				chan<- message
	nodeHealth				map[string]int
	nodeHealthLast30Ticks	map[string][]uint
	nodeHealthLastPing		map[string]uint
	rwmux					sync.RWMutex
	
	resourceControllers		map[uint]interface{}
	resCtl_lvd				*lvd
	resourcePlacementAndState	map[string]Cluster_resource
//	_vote_delay	chan int
	}


var b *Brain


func NewBrain(a_ex_brn <-chan message, a_brn_ex chan<- message) *Brain {
	b := Brain{
		isMaster:			false,
		masterNode:			nil,
		killBrain:			false,
		nominatedBy:		make(map[string]bool),
		voteCounterExists:	false,
		quorum:				0,
		
		ex_brn:					a_ex_brn,
		brn_ex:					a_brn_ex,
		nodeHealth:				make(map[string]int),
		nodeHealthLast30Ticks:	make(map[string][]uint),
		nodeHealthLastPing:		make(map[string]uint),
		
		resourceControllers:	make(map[uint]interface{}),
		rwmux:					sync.RWMutex{},
//		_vote_delay				make(chan int),
		}
//	if config.enabledResourceControllers[resource_controller_id_libvirt] {
//		b.resourceControllers[resource_controller_id_libvirt] = NewLVD()
//		if b.resourceControllers[resource_controller_id_libvirt] == nil {
//			lg.msg("ERROR, NewLVD libvirt resource controller failed to start")}}
	if config.enabledResourceControllers[resource_controller_id_libvirt] {
		b.resCtl_lvd = NewLVD()
		if b.resCtl_lvd == nil {
			lg.msg("ERROR, NewLVD libvirt resource controller failed to start")}}
		
	go b.updateNodeHealth()
	go b.messageHandler()
	go b.getMasterNode()
//	go b.resourceBalancer()
	return &b}

func (b *Brain)KillBrain(){
	b.killBrain=true}

func  (b *Brain)messageHandler(){
	var m message
	for {
		m=message{}
		if b.killBrain == true{
			return}
		m = <-b.ex_brn
		//if config.DebugNetwork{
		//	fmt.Printf("DEBUG BRAIN received message %+v\n", m)}
		lg.msg_debug(fmt.Sprintf("DEBUG BRAIN received message %+v\n", m),3)
		
		if m.RpcFunc == brainRpcAskForMasterNode && m.SrcHost != config._MyHostname() {
			b.replyToAskForMasterNode(&m)
			continue
		
		// Master node only replies
		}else if b.isMaster &&
				(m.RpcFunc == brainRpcElectAsk ||
				 m.RpcFunc == brainRpcElectNominate) {
				b.SendMsg(m.SrcHost, brainRpcHaveMasterNodeReply, config.MyHostname)
				continue
		// respond to message with info about master node
		}else if m.RpcFunc == brainRpcHaveMasterNodeReply {
			// TODO remove, it's only for debug function using fmt.Printf
			// makes a copy of this string
			// I had some strange problems with fmt.Printf without creating string copy
			str := m.Argv[0]
			b.masterNode=&str
			//b.masterNode=&m.Argv[0]
			lg.msg(fmt.Sprintf("got master node %s from host %s",m.Argv[0],m.SrcHost))
		// respond to brainRpcHaveMasterNodeReplyNil
		}else if m.RpcFunc == brainRpcHaveMasterNodeReplyNil {
			b.masterNode = nil
			b.SendMsg("__everyone__",brainRpcElectAsk,"asking for elections")
			continue
		// respond to request for elections
		}else if m.RpcFunc == brainRpcElectAsk {
			b.vote()
			continue
		// respond to master node nomination
		}else if m.RpcFunc == brainRpcElectNominate && 
				m.SrcHost != config._MyHostname() {
			if config.MyHostname == m.Argv[0] {
				//this node got nominated
				b.nominatedBy[m.SrcHost]=true
				//go b.countVotes()}}}}
				fmt.Println("DEBUG ",m)
				go b.countVotes()}}}}

func (b *Brain)vote(){
	hostname := b.findHighWeightNode()
	nm := brainNewMessage()
	nm.DestHost = "__everyone__"
	nm.RpcFunc=brainRpcElectNominate
	nm.Argc=2
	nm.Argv=make([]string,2)
	nm.Argv[0]=*hostname
	nm.Argv[1]=fmt.Sprintf("nominating node %s",*hostname)
	b.brn_ex <- *nm}

func (b *Brain)countVotes(){
	if b.voteCounterExists {
		lg.msg_debug("received ask for vote, but vote coroutine is already running",1)
		return}
	b.voteCounterExists = true
	config.ClusterTick_sleep()
	var sum uint = 0
	for k,v := range b.nominatedBy {
		lg.msg(fmt.Sprintf("this node (%s) nominated by %s %t",config.MyHostname,k,v))
		if v {
			sum++}}
	// to get quorum host needs quorum-1 votes (it doesn't need it's own vote )
	if sum >= config.Quorum-1 {
		fmt.Printf("this host won elections with %d votes (quorum==%d) of votes",sum,config.Quorum)
		b.isMaster = true
		b.masterNode = &config.MyHostname
		b.nominatedBy = make(map[string]bool)
		//fmt.Println("DEBUG WON ", b.isMaster, *b.masterNode, b.nominatedBy)
	}else{
		lg.msg(fmt.Sprintf(
			"elections failed, not enough votes (votes=%d) (quorum==%d), %+v",
			sum,config.Quorum,b.nominatedBy))}
	b.nominatedBy = make(map[string]bool)
	b.voteCounterExists = false}

func (b *Brain)replyToAskForMasterNode(m *message){
	if b.isMaster && *b.masterNode == config.MyHostname {
		b.SendMsg(m.SrcHost, brainRpcHaveMasterNodeReply, config.MyHostname)
	}else if b.masterNode == nil {
		b.SendMsg(m.SrcHost,
			brainRpcHaveMasterNodeReplyNil,
			"cluster Doesn't currently have a masterNode")}}

//this function is very naive
//node health is calculated form average of last 30 check intervals
//this code is not very optimized, but it's good enough
func (b *Brain)updateNodeHealth(){	//TODO, add node load to health calculation
	//var dt time.Duration
	var sum uint
	var avg float32
	for{
		if b.killBrain {
			return}
		//make new map, it's safer than removing dropped nodes from old map
		//lock mutex for writing
		b.rwmux.Lock()
		b.nodeHealth = make(map[string]int)
		b.nodeHealth[config.MyHostname]=HealthGreen
		for k,v := range e.GetHeartbeat() {
			//get absolute value of time.
			//in this simple implemetation time can be negative due to time differences on host
			dt := time.Now().Sub(*v)
			//get absolute value
			if dt < 0 {
				dt = 0 - dt}
			//set last ping value
			b.nodeHealthLastPing[k]=uint(dt / time.Millisecond)
			//add health value to list
			if dt> (time.Millisecond * time.Duration(config.NodeHealthCheckInterval * 2)){
				b.nodeHealthLast30Ticks[k] = append(b.nodeHealthLast30Ticks[k], HealthRed)
			}else if dt > (time.Millisecond * time.Duration(config.NodeHealthCheckInterval)) {
				b.nodeHealthLast30Ticks[k] = append(b.nodeHealthLast30Ticks[k], HealthOrange)
			}else {
				b.nodeHealthLast30Ticks[k] = append(b.nodeHealthLast30Ticks[k], HealthGreen)
				}
			sum=0
			for _,x := range b.nodeHealthLast30Ticks[k] {
				sum = sum + x}
			avg = float32(sum) / float32(len(b.nodeHealthLast30Ticks[k]))
			if avg>2 {
				b.nodeHealth[k]=HealthRed
			}else if avg>1 && avg<=2 {
				b.nodeHealth[k]=HealthOrange
			}else if avg >= 1 && avg <= 1.1 {
				b.nodeHealth[k]=HealthGreen}
			//remove last position if slice size gets over 29
			if len(b.nodeHealthLast30Ticks[k]) > 29 {
				b.nodeHealthLast30Ticks[k] = b.nodeHealthLast30Ticks[k][1:]}}
		// updating quorum stats
		sum=0
		for _,v := range b.nodeHealth {
			if v <= HealthOrange {
				sum++}
			}
		b.quorum = sum
		// remove master node if quorum falls below config
		if b.quorum < config.Quorum {
			b.masterNode = nil
			b.isMaster = false}
		// remove master node if it's health gets above orange
		// separate if for readability
		if b.masterNode != nil && b.nodeHealth[*b.masterNode] >= HealthOrange {
			b.masterNode = nil
			b.isMaster = false}
		//unlock writing mutex 
		b.rwmux.Unlock()
		time.Sleep(time.Millisecond * time.Duration(config.NodeHealthCheckInterval))}
		}

func (b *Brain)findHighWeightNode() *string {
	var host *string
	var hw uint = 0
	//lock read mutex
	b.rwmux.RLock()
	for k,v := range b.nodeHealth{
		n := config.GetNodebyHostname(&k)
		if v == HealthGreen && n != nil && n.Weight > hw {
			hw=n.Weight
			host=&n.Hostname}}
	//unlock read mutex
	b.rwmux.RUnlock()
	return host}

func brainNewMessage() *message {
	var m = message{
		SrcHost: config.MyHostname,
		SrcMod: msgModBrain,
		DestMod: msgModBrain,
		Time: time.Now(),
		Argc: 1,
		Argv: make([]string,1)}
	return &m}

func (b *Brain)getMasterNode(){
	for{
		if b.killBrain {
			return}
		config.ClusterTick_sleep()
		if b.masterNode == nil {
			lg.msg("looking for master node")
			b.SendMsg("__everyone__", brainRpcAskForMasterNode, "asking for master node")
			config.ClusterTick_sleep()}}}

func (b *Brain)SendMsg(host string, rpc uint, str string){
	var m *message = brainNewMessage()
	m.DestHost=host
	m.RpcFunc=rpc
	m.Argv = []string{str}
	b.brn_ex <- *m}
	
func (b *Brain)SendMsgINT(host string, rpc uint, str string, c1 interface{}){
	var m *message = brainNewMessage()
	m.DestHost=host
	m.RpcFunc=rpc
	m.Argv = []string{str}
	m.custom1 = c1
	b.brn_ex <- *m}
	
func (b *Brain)PrintNodeHealth(){
	fmt.Printf("=== Node Health ===\n")
	if b.masterNode != nil {
		fmt.Printf("Master node: %s\n", *b.masterNode)
	}else{
		fmt.Printf("No master node\n")}
	fmt.Printf("Quorum: %d\n", b.quorum)
	//read lock mutex for nodeHealth maps
	b.rwmux.RLock()
	for k,v := range b.nodeHealth {
		switch v {
			case HealthGreen:
				fmt.Printf("node: %s, last_msg: %dms, health: %s %+v\n",k,b.nodeHealthLastPing[k],"Green",b.nodeHealthLast30Ticks[k])
			case HealthOrange:
				fmt.Printf("node: %s, last_msg: %dms, health: %s %+v\n",k,b.nodeHealthLastPing[k],"Orange",b.nodeHealthLast30Ticks[k])
			case HealthRed:
				fmt.Printf("node: %s, last_msg: %dms, health: %s %+v\n",k,b.nodeHealthLastPing[k],"Red",b.nodeHealthLast30Ticks[k])}}
	//unlock mutex for nodeHealth maps
	b.rwmux.RUnlock()
	fmt.Printf("===================\n")}

func (b *Brain)reportControllerResourceState(ctl ResourceController) {
	var cl_res *[]Cluster_resource
	var cl_utl *[]Cluster_utilization
	cl_res = ctl.Get_running_resources()
		if cl_res == nil {
			lg.msg("ERROR ,BRAIN, get_running_resources returned NULL pointer")}
	b.SendMsgINT("__master__", brianRpcSendingClusterResources, "sending Cluster_resources to Master node", *cl_res)
	
	cl_utl = ctl.Get_utilization()
		if cl_utl == nil {
			lg.msg("ERROR ,BRAIN, get_running_resources returned NULL pointer")}
	b.SendMsgINT("__master__", brianRpcSendingClusterResources, "sending clsuter_utilization to Master node", *cl_utl)
	}

func (b *Brain)resourceBalancer(){
	for{
		if(b.killBrain){
			return}
		
		b.reportControllerResourceState(b.resCtl_lvd)
		
		if(b.isMaster){
			// TODO
			// get resource states
			
			// generate resource placement plan
			
			// change resource states on nodes
			}
		config.ClusterTick_sleep()}}

func (b *Brain)is_this_node_a_master() bool {
	return b.isMaster }

func (b *Brain)getMasterNodeName() *string {
	return b.masterNode }
