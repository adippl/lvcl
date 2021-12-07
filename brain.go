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
import "strings"

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
	brn_ex					chan<- message
	nodeHealth				map[string]int
	nodeHealthLast30Ticks	map[string][]uint
	nodeHealthLastPing		map[string]uint
	rwmux					sync.RWMutex
	
	//resourceControllers		map[uint]interface{}
	resCtl_lvd					*lvd
	resCtl_Dummy				*Dummy_rctl
	Epoch						uint64
	desired_resourcePlacement	[]Cluster_resource
	current_resourcePlacement	[]Cluster_resource
	local_resourcePlacement		[]Cluster_resource
	rwmux_locP					sync.RWMutex
	rwmux_curPlacement			sync.RWMutex
	expectedResUtil				[]Node
	lastPickedNode				string
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
		Epoch:			0,
		nodeHealth:				make(map[string]int),
		nodeHealthLast30Ticks:	make(map[string][]uint),
		nodeHealthLastPing:		make(map[string]uint),
		resCtl_lvd:				nil,
		resCtl_Dummy:			nil,
		
		//resourceControllers:	make(map[uint]interface{}),
		rwmux:					sync.RWMutex{},
		rwmux_locP:				sync.RWMutex{},
		rwmux_curPlacement:		sync.RWMutex{},
//		_vote_delay				make(chan int),
		}
//	if config.enabledResourceControllers[resource_controller_id_libvirt] {
//		b.resourceControllers[resource_controller_id_libvirt] = NewLVD()
//		if b.resourceControllers[resource_controller_id_libvirt] == nil {
//			lg.msg("ERROR, NewLVD libvirt resource controller failed to start")}}
	if config.EnabledResourceControllers[resource_controller_id_libvirt] {
		b.resCtl_lvd = NewLVD()
		if b.resCtl_lvd == nil {
			lg.msg(
				"ERROR, NewLVD libvirt resource controller failed to start")}}


	if config.EnabledResourceControllers[resource_controller_id_dummy] {
		b.resCtl_Dummy = NewDummy()
		if b.resCtl_Dummy == nil {
			lg.msg(
				"ERROR, NewDummy dummy resource controller failed to start")}}
		
	go b.messageHandler()
	lg.msg_debug(3, "brain started messageHandler()")
	go b.updateNodeHealth()
	lg.msg_debug(3, "brain started updateNodeHealth()")
	go b.getMasterNode()
	lg.msg_debug(3, "brain started getMasterNode()")

	go b.LogBrainStatus()
	go b.resourceBalancer()
	return &b}

func (b *Brain)KillBrain(){
	b.killBrain=true
	if config.DebugLevel>2 {
		fmt.Println("Debug, KillBrain()")}}

func  (b *Brain)messageHandler(){
	var m message
	for {
		m=message{}
		if b.killBrain == true{
			return}
		m = <-b.ex_brn
		if config.DebugNetwork{
			lg.msg_debug(1, fmt.Sprintf(
				"DEBUG BRAIN received message %+v\n", m))}
			//fmt.Printf("DEBUG BRAIN received message %+v\n", m)}}
		
		
		//handle client's requests for status
		if b.msg_handle_clientAskAboutStatus(&m) {continue}
		
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
		lg.msg_debug(1, 
			"received ask for vote, but vote coroutine is already running")
		return}
	b.voteCounterExists = true
	config.ClusterTick_sleep()
	var sum uint = 0
	for k,v := range b.nominatedBy {
		lg.msg(fmt.Sprintf(
			"this node (%s) nominated by %s %t",
			config.MyHostname,k,v))
		if v {
			sum++}}
	// to get quorum host needs quorum-1 votes (it doesn't need it's own vote )
	if sum >= config.Quorum-1 {
		lg.msg_debug(
			2,
			fmt.Sprintf(
				"this host won elections with %d votes (quorum==%d) of votes",
				sum,
				config.Quorum))
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
			// get absolute value of time.
			// in this simple implemetation time can be negative due to time 
			// differences on host
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
		time.Sleep(
			time.Millisecond * time.Duration(
				config.NodeHealthCheckInterval))}}

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
	
func (b *Brain)writeNodeHealth() string {
	var sb strings.Builder
	sb.WriteString("\n=== Node Health ===\n")
	if b.masterNode != nil {
		sb.WriteString(fmt.Sprintf("Master node: %s\n", *b.masterNode))
	}else{
		sb.WriteString("No master node\n")}
	sb.WriteString(fmt.Sprintf("Quorum: %d\n", b.quorum))
	//read lock mutex for nodeHealth maps
	b.rwmux.RLock()
	for k,v := range b.nodeHealth {
		switch v {
			case HealthGreen:
				sb.WriteString(fmt.Sprintf(
					"node: %s, last_msg: %dms, health: %s %+v\n",
					k,
					b.nodeHealthLastPing[k],
					"Green",
					b.nodeHealthLast30Ticks[k]))
			case HealthOrange:
				sb.WriteString(fmt.Sprintf(
					"node: %s, last_msg: %dms, health: %s %+v\n",
					k,
					b.nodeHealthLastPing[k],
					"Orange",
					b.nodeHealthLast30Ticks[k]))
			case HealthRed:
				sb.WriteString(fmt.Sprintf(
					"node: %s, last_msg: %dms, health: %s %+v\n",
					k,
					b.nodeHealthLastPing[k],
					"Red",
					b.nodeHealthLast30Ticks[k]))}}
	//unlock mutex for nodeHealth maps
	b.rwmux.RUnlock()
	
	sb.WriteString(fmt.Sprintf(
		"===-- Cluster Resources (epoch %d) --===\n", config.GetEpoch()))
	config.rwmux.RLock()
	for _,v := range config.Resources {
		sb.WriteString(fmt.Sprintf("Id %d\tdesState %s\tName %s\n", 
			v.Id, v.StateString(), v.Name))}
	config.rwmux.RUnlock()

	sb.WriteString("===================\n")
	return sb.String()}

func (b *Brain)PrintNodeHealth(){
	lg.msg(b.writeNodeHealth())}

func (b *Brain)reportControllerResourceState(ctl ResourceController) {
	var cl_res *[]Cluster_resource
	var cl_utl *[]Cluster_utilization
	cl_res = ctl.Get_running_resources()
		if cl_res == nil {
			lg.msg(
				"ERROR ,BRAIN, get_running_resources returned NULL pointer")}
	b.SendMsgINT(
		"__master__",
		brianRpcSendingClusterResources,
		"sending Cluster_resources to Master node",
		*cl_res)
	
	cl_utl = ctl.Get_utilization()
		if cl_utl == nil {
			lg.msg(
				"ERROR ,BRAIN, get_running_resources returned NULL pointer")}
	b.SendMsgINT(
		"__master__", 
		brianRpcSendingClusterResources,
		"sending clsuter_utilization to Master node",
		*cl_utl)
	}

func (b *Brain)resourceBalancer(){
	//time.Sleep(time.Duration(5) *time.Second)
	//debug on local machine
	var debug_local bool = false

	if config._MyHostname() == "x270" {
		debug_local = true}
	for{
		if(b.killBrain){
			return}
		
		//b.reportControllerResourceState(b.resCtl_lvd)
		b.update__local_resourcePlacement()
		b.update_expectedResUtil()
		if debug_local {
			b.basic_placeResources()}
		if(b.isMaster){
			// TODO
			// get resource states
			
			// generate resource placement plan
			if ! debug_local {
				b.basic_placeResources()}
			
			// change resource states on nodes
			}
		config.ClusterTick_sleep()}}

func (b *Brain)is_this_node_a_master() bool {
	return b.isMaster }

func (b *Brain)getMasterNodeName() *string {
	return b.masterNode }

func (b *Brain)msg_handle_clientAskAboutStatus(m *message) bool{
	var reply message = *brainNewMessage()
	if	m.RpcFunc == clientAskAboutStatus &&
		m.DestMod == msgModBrain {
		
		reply.DestHost = m.SrcHost
		reply.DestMod = msgModClient
		reply.RpcFunc = clientPrintTextStatus
		reply.Argv = []string{
			b.writeNodeHealth(),
			}
		b.brn_ex <- reply
		return true}
	return false}

func (b *Brain)LogBrainStatus(){
	for{
		lg.msg(*b.writeBrainStatus())
		time.Sleep(time.Duration(5) * time.Second)}}

func (b *Brain)writeBrainStatus() *string {
	var sb strings.Builder
	var retString string
	sb.WriteString("\n====== desired resource placement ======\n")
	for _,v:=range b.desired_resourcePlacement {
		sb.WriteString(fmt.Sprintf("ctl %s\tstate %s\tnode %s\tname %s\n",
			v.CtlString(), v.StateString(), v.placement, v.Name ))}
	sb.WriteString("========================================\n")
	
	sb.WriteString("\n====== current resource placement ======\n")
	for _,v:=range b.current_resourcePlacement {
		sb.WriteString(fmt.Sprintf("ctl %s\tstate %s\tnode %s\tname %s\n",
			v.CtlString(), v.StateString(), v.placement, v.Name ))}
	sb.WriteString("========================================\n")
		
	sb.WriteString("\n======= local resource placement =======\n")
	for _,v:=range b.local_resourcePlacement {
		sb.WriteString(fmt.Sprintf("ctl %s\tstate %s\tnode %s\tname %s\n",
			v.CtlString(), v.StateString(), v.placement, v.Name ))}
	sb.WriteString("========================================\n")
	sb.WriteString(*b.writeSum_expectedResUtil())
		
	retString = sb.String()
	return &retString}

func (n *Node)doesUtilFitsOnNode(u *Cluster_utilization) (bool,bool,float32) {
	var hwFits bool = false
	var usageFits bool = false
	var hw uint64 = 0
	var usage uint64 = 0
	var usagePerc float32 = 0

	for k,_:=range n.HwStats {
		if	u.Id == n.HwStats[k].Id {
			if u.Value < n.HwStats[k].Value {
				hwFits = true
				hw = n.HwStats[k].Value
				break}}}
	if hwFits {
		for k,_:=range n.Usage {
			if	u.Id == n.Usage[k].Id {
				if u.Value < ( hw - n.Usage[k].Value ) {
					usageFits = true
					usage = n.Usage[k].Value
					break}}}}
	if hw != 0 {
		usagePerc = float32(usage/hw)
	}else{
		usagePerc = -1.0}
//	if ! (hwFits || usageFits) {
//		fmt.Printf("res DOES NOT FIT ON THE NODE %b %b|%s|%s|%s|%d|%+v\n",
//			hwFits,
//			usageFits,
//			n.Hostname,
//			u.Name,
//			u.NameString(),
//			u.Value,
//			u)}
	return hwFits, usageFits, usagePerc}

func (n *Node)doesResourceFitsOnNode(r *Cluster_resource) bool {
	for _,v:=range r.Util {
		if _,does_fit,_ := n.doesUtilFitsOnNode(&v); does_fit == false {
			return false}}
	return true}

func (b *Brain)update_expectedResUtil(){
	var node *Node = nil
	var util *Cluster_utilization = nil
	config.rwmux.RLock()
	//create copy, it's safer* (* not really)
	b.expectedResUtil = config.Nodes
	
	config.rwmux.RUnlock()
	for k,_:=range b.expectedResUtil {
		
		//deep copy 
		b.expectedResUtil[k].Usage = make([]Cluster_utilization, 0,
			len(b.expectedResUtil[k].HwStats))
		for kk,_:=range b.expectedResUtil[k].HwStats {
			b.expectedResUtil[k].Usage = append(
				b.expectedResUtil[k].Usage,
				b.expectedResUtil[k].HwStats[kk])}
		
		//zero all resource usage
		for kk,_:=range b.expectedResUtil[k].Usage {
			b.expectedResUtil[k].Usage[kk].Value = 0}}
	
	for x,_:=range b.desired_resourcePlacement {
		//get ptr to node
		node = b.getPtrToNode(&b.desired_resourcePlacement[x].placement)
		for y,_:=range b.desired_resourcePlacement[x].Util {
			//get ptr to Cluster_utilization if exists on this node
			util = node.getPtrToUtil(b.desired_resourcePlacement[x].Util[y].Id)
			if ! util.UtilAdd(&b.desired_resourcePlacement[x].Util[y]) {
				continue}}}}

func (n *Node)getPtrToUtil(id int) *Cluster_utilization {
	for k,_:=range n.Usage {
		if n.Usage[k].Id == id {
			return &n.Usage[k]}}
	return nil}

func (b *Brain)getPtrToNode(name *string) *Node {
	for k,_:=range b.expectedResUtil {
		if b.expectedResUtil[k].Hostname == *name {
			return &b.expectedResUtil[k]}}
	return nil}

// modifies value of *lastNode
func (b *Brain)_place_single_resource(
	lastNode *int, res *Cluster_resource) bool {
	
	var nodeArrSize int = 0
	var nodesChecked int = 0
	var resCopy Cluster_resource
	
	nodeArrSize = len(config.Nodes)
	//increment lastNode to avoild placing in the same node as last resource
	*lastNode++
	if *lastNode > nodeArrSize-1 {
		//reset nodeArray index to 0, we want to loop over this array
		*lastNode = 0}
	
	//for config.Nodes[ *lastNode ].doesResourceFitsOnNode( res ) == false {
	for b.expectedResUtil[ *lastNode ].doesResourceFitsOnNode( res ) == false {
		//checked on all nodes?
		nodesChecked++
		if nodesChecked >= nodeArrSize {
			// all nodes checked, res coulsn't be placed
//			lg.msg_debug(5, fmt.Sprintf( "_place_single_resource looped, (%d >= %d) = %b", nodesChecked, nodeArrSize, nodesChecked >= nodeArrSize))
			return false}
		if *lastNode >= nodeArrSize-1 {
			//reset nodeArray index to 0, we want to loop over this array
			*lastNode = 0
			continue}
		*lastNode++}
	//found fitting node, placing
	//copy because we need to modify struct modify
	resCopy = *res
	resCopy.placement = config.Nodes[ *lastNode ].Hostname
	b.desired_resourcePlacement = append(b.desired_resourcePlacement, 
	resCopy)
	//resource placed in desired resources
	return true}
		
	

func (b *Brain)basic_placeResources(){
	var lastNode int = 0
	config.rwmux.RLock()
	for k,_:=range config.Resources {
		//nodesChecked = 0
		if config.Resources[k].State != resource_state_running {
			// skip if resource turned off in config
			continue}
		if b.checkIfResourceHasPlacement(&config.Resources[k]) {
			continue}
				
		if b._place_single_resource(&lastNode, &config.Resources[k]){
			continue
		}else{
			lg.msg_debug(2, fmt.Sprintf(
				"resource %s couldn't be placed on any node",
				config.Resources[k].Name))}}
	config.rwmux.RUnlock()
	}


func (b *Brain)checkIfResourceHasPlacement(r *Cluster_resource) bool {
	for k,_:=range b.desired_resourcePlacement {
		if	b.desired_resourcePlacement[k].Name == r.Name &&
			b.desired_resourcePlacement[k].Id == r.Id {
			return true}}
	return false}


func (b *Brain)checkIfResourceIsCurrentlyPlaced(r *Cluster_resource) bool {
	for k,_:=range b.current_resourcePlacement {
		if	b.current_resourcePlacement[k].Name == r.Name &&
			b.current_resourcePlacement[k].Id == r.Id {
			return true}}
	return false}

func (b *Brain)checkIfResourceIsPlacedLocally(r *Cluster_resource) bool {
	for k,_:=range b.local_resourcePlacement {
		if	b.local_resourcePlacement[k].Name == r.Name &&
			b.local_resourcePlacement[k].Id == r.Id {
			return true}}
	return false}

func (b *Brain)writeSum_expectedResUtil() *string {
	var sb strings.Builder
	var retStr string
	sb.WriteString("\n=== Node Util ===\n")

	for k,_:=range b.expectedResUtil {
		sb.WriteString(fmt.Sprintf("== node %s ==\n",
			b.expectedResUtil[k].Hostname))
		//assuming that Node.Util[] is the same order and length as
		//node.HwStats
		//IT SHOULD BE
		for kk,_:=range b.expectedResUtil[k].Usage {
			sb.WriteString(fmt.Sprintf("node %s\t%d\t/\t%d\n",
				b.expectedResUtil[k].Usage[kk].NameString(),
				b.expectedResUtil[k].Usage[kk].Value,
				b.expectedResUtil[k].HwStats[kk].Value))}}
	sb.WriteString("\n=================\n")
	retStr = sb.String()
	return &retStr}
	
func (b *Brain)update__local_resourcePlacement(){
	var r_arr []Cluster_resource
	var retRes []Cluster_resource = make([]Cluster_resource,0)
	
	if b.resCtl_lvd != nil {
		r_arr = *b.resCtl_lvd.Get_running_resources()
		if r_arr != nil {
			for k,_:=range r_arr {
				retRes = append(retRes, r_arr[k])}}}
	if b.resCtl_Dummy != nil {
		//TODO handle lvd
		r_arr = *b.resCtl_Dummy.Get_running_resources()
		if r_arr != nil {
			for k,_:=range r_arr {
				retRes = append(retRes, r_arr[k])}}}
	
	b.rwmux_locP.Lock()
	b.local_resourcePlacement = retRes
	b.rwmux_locP.Unlock()}


func (b *Brain)IncEpoch() {
	b.rwmux.Lock()
	b.Epoch++
	b.rwmux.Unlock()}

func (b *Brain)GetEpoch() uint64 {
	var e uint64
	b.rwmux.RLock()
	e = b.Epoch
	b.rwmux.RUnlock()
	return e}

func (b *Brain)isTheirEpochBehind(i uint64) bool {
	var r bool
	b.rwmux.RLock()
	r = (i < b.Epoch)
	b.rwmux.RUnlock()
	return r}

func (b *Brain)isTheirEpochAhead(i uint64) bool {
	var r bool
	b.rwmux.RLock()
	r = (i > b.Epoch)
	b.rwmux.RUnlock()
	return r}


func (b *Brain)epochSender(){
	var m message
	var t time.Time
	lg.msg_debug(3, "exchange launched configEpochSender()")
	for{
		if b.killBrain { //ugly solution
			return}
		t = time.Now()
		
		m = message{
			SrcHost: config.MyHostname,
			DestHost: "__everyone__",
			SrcMod: msgModBrain,
			DestMod: msgModBrain,
			RpcFunc: brainNotifAboutEpoch,
			Time: t,
			Argc: 1,
			Argv: []string{"epoch"},
			Cuint: b.GetEpoch(),
			}
		b.brn_ex <- m
		time.Sleep(time.Millisecond * time.Duration(config.HeartbeatInterval))}}

