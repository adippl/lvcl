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

import "net"
import "fmt"
import "time"
import "sync"
import "math/rand"

var e *Exchange

type Exchange struct{
	nodeList	*[]Node
	outgoing		map[string]*eclient
	incoming		map[string]*eclient
	usocks			map[string]*eclient
	listenTCP	net.Listener
	listenUnix	net.Listener
	clientMap		map[string]string
	
	heartbeatLastMsg		map[string]*time.Time
	heartbeatDelta		map[string]*time.Duration
	
	recQueue	chan message
	loc_ex		chan message
	brn_ex		<-chan message
	ex_brn		chan<- message
	log_ex		<-chan message
	ex_log		chan<- message
	
	killExchange	bool // ugly solution
	rwmux			sync.RWMutex
	rwmuxUSock		sync.RWMutex
	
	confFileHash string `json:"-"`
	confHashCheck	bool `json:"-"` 
	}


func NewExchange(	a_brn_ex <-chan message,
					a_ex_brn chan<- message,
					a_log_ex <-chan message,
					a_ex_log chan<- message) *Exchange {
	e := Exchange{
		nodeList:		&config.Nodes,
		outgoing:		make(map[string]*eclient),
		incoming:		make(map[string]*eclient),
		heartbeatLastMsg:	make(map[string]*time.Time),
		heartbeatDelta:	make(map[string]*time.Duration),
		clientMap:		make(map[string]string),
		killExchange:	false,
		recQueue:		make(chan message),
		loc_ex:			make(chan message),
		brn_ex:			a_brn_ex,
		ex_brn:			a_ex_brn,
		log_ex:			a_log_ex,
		ex_log:			a_ex_log,
		rwmux:			sync.RWMutex{},
		rwmuxUSock:		sync.RWMutex{},
		confFileHash:	config.GetField_string("ConfFileHash"),
		confHashCheck:	config.GetField_bool("ConfHashCheck"),
		}
	// don't log before forwarder is started
	go e.forwarder()
	lg.msg_debug(3, "exchange launched forwarder()")
	go e.initListenTCP()
	go e.initListenUnix()
	go e.reconnectLoop()
	go e.sorter()
	go e.heartbeatSender()
	go e.updateNodeHealthDelta()
	lg.msg_debug(2, "exchange launched all it's goroutines")
	return &e}

func (e *Exchange)updateNodeHealthDelta(){
	lg.msg_debug(3, "exchange launched updateNodeHealthDelta()")
	if e.killExchange {
		return}
	for{
		// creating new map is much easier than searching for missing nodes
		e.heartbeatDelta=make(map[string]*time.Duration)
		for k,v := range e.heartbeatLastMsg {
			dt := time.Now().Sub(*v)
			e.heartbeatDelta[k]=&dt}
		time.Sleep(time.Millisecond * time.Duration(config.ClusterTickInterval))}}

func (e *Exchange)initListenTCP(){
	var err error
	lg.msg_debug(3, "exchange launched initListenTCP()")
	e.listenTCP, err = net.Listen("tcp", ":" + config.TCPport)
	if err != nil {
		lg.err("ERR, Couldn't open TCP Socket",err)
		panic(err)}
	for{
		if e.killExchange { //ugly solution
			return}
		conn,err := e.listenTCP.Accept()
		if err != nil {
			lg.err("ERR, net.Listener.Accept() ",err)}
		
		raddr := conn.RemoteAddr().String()
		laddr := conn.LocalAddr().String()
		lg.msg(fmt.Sprintf("info, %s connected to node %s ", raddr, laddr))
		
		ec := &eclient{
			hostname:	conn.RemoteAddr().String(),
			incoming:	e.recQueue,
			conn:		conn,}
		go ec.listen()}}

func (e *Exchange)initListenUnix(){
	var err error
	var sockId uint
	var s_usockID string
	e.listenUnix, err = net.Listen("unix", config.UnixSocket)
	if err != nil {
		lg.err("ERR, couldn't open Unix Socket",err)
		panic(err)}
	for{
		if e.killExchange { //ugly solution
			return}
		conn,err := e.listenUnix.Accept()
		if err != nil {
			lg.err("ERR, net.Listener.Accept() ",err)}
		sockId = uint(rand.Uint32())
		s_usockID = fmt.Sprintf("usock_%d", sockId)
		lg.msg_debug(1, fmt.Sprintf(
			"Received connection to Unix Socket, asigning sock id=%d", sockId))
		
		ec := &eclient{
			usock:		true,
			hostname:	s_usockID,
			incoming:	e.recQueue,
			outgoing:	make(chan message),
			exch:		e,
			conn:		conn,}
		go ec.listen()
		go ec.forward()
		ec.sendUsockClientID(sockId)
		//mutex
		e.rwmux.Lock()
		e.outgoing[ec.hostname]=ec
		e.clientMap[s_usockID]=config._MyHostname()
		e.rwmux.Unlock()
		//mutex
		e.notifyClusterAboutClient(s_usockID, config._MyHostname())
		}
	}

func (e *Exchange)reconnectLoop(){
	lg.msg_debug(3, "exchange launched reconnectLoop()")
	for{
		if e.killExchange { //ugly solution
			return}
		//fmt.Println("e.killExchange ", e.killExchange)
		e.rwmux.RLock()
		for _,n := range *e.nodeList{
			if e.outgoing[n.Hostname] == nil && 
				n.Hostname != config.MyHostname {
				
				lg.msg_debug(1, fmt.Sprintf(
					"attempting to recconect to host %s",n.Hostname))
				
				go e.startConn(n)}}
		e.rwmux.RUnlock()
		time.Sleep(
			time.Millisecond * time.Duration(config.ReconnectLoopDelay))}}


func (e *Exchange)startConn(n Node){	//TODO connect to eclient
	if(n.Hostname == config.MyHostname){
		return}
	c,err := net.Dial("tcp",n.NodeAddress)
	if(err!=nil){
		lg.msg(fmt.Sprintf("ERR, dialing %s error: \"%s\"", n.Hostname, err))
		e.rwmux.Lock()
		e.outgoing[n.Hostname]=nil //just to be sure
		e.rwmux.Unlock()
		return}
	ec := eclient{
		hostname:		n.Hostname,
		originLocal:	true,
		outgoing:		make(chan message),
		conn:			c,
		exch:			e,
		}
	go ec.forward();
	e.rwmux.Lock()
	e.outgoing[n.Hostname]=&ec
	e.rwmux.Unlock()
	}

func (e *Exchange)forwarder(){
	var m *message
	var temp_m message
	var brnOk, logOk, locOk bool
	for{
		brnOk=true
		logOk=true
		locOk=true
		m=nil
		if e.killExchange { //ugly solution
			return}
		
		select{
			case temp_m,brnOk = <-e.brn_ex:
				m=&temp_m
			case temp_m,logOk = <-e.log_ex:
				m=&temp_m
			case temp_m,locOk = <-e.loc_ex:
				m=&temp_m
			}
		if !( brnOk && logOk && locOk ) { 
			fmt.Printf("\nWARNING, all exchange channels are closed brnOk=%b logOk=%b locOk=%b\n", brnOk, logOk, locOk)
			fmt.Printf("channels closedr Killing exchange\n")
			e.KillExchange()
			return}
		
		e.markMessageWithConfigHash(m)
		
		if config.DebugNetwork {
			fmt.Printf("DEBUG forwarder recieved %+v\n", m)}
		e.rwmux.RLock()
		if	m.SrcHost == config.MyHostname &&
			config.GetNodebyHostname(&m.DestHost) != nil &&
			e.outgoing[m.DestHost] != nil {
			e.outgoing[m.DestHost].outgoing <- *m
			e.rwmux.RUnlock()
		}else{
			e.rwmux.RUnlock()
			if(m.DestHost!="__everyone__"){
			fmt.Printf("DEBUG forwarder recieved INVALID message %+v\n", m)}
			}
		
		//forward to everyone else
		if	m.SrcHost == config.MyHostname && m.DestHost == "__everyone__" {
			for _,n := range config.Nodes{
				e.rwmux.RLock()
				if n.Hostname != config.MyHostname && e.outgoing[n.Hostname] != nil {
					if config.DebugNetwork {
						fmt.Printf("DEBUG forwarder pushing to %s  %+v\n", n.Hostname, m)}
					e.outgoing[n.Hostname].outgoing <- *m }
					e.rwmux.RUnlock()}}}}

func (e *Exchange)sorter(){
	var m message
	var recOpen bool
	lg.msg_debug(3, "exchange launched sorter()")
	for{
		if e.killExchange { //ugly solution
			return}
		m = message{}
		m,recOpen = <-e.recQueue
		if ! recOpen {
			//channel closed, return 
			return}
		if config.DebugNetwork {
			fmt.Printf("DEBUG SORTER received %+v\n", m)}
		
		// check if message comes with  
		if ! m.verifyMessageConfigHash() {
			lg.msg_debug(1,
				fmt.Sprintf("exchange: validation config hash: %+v", m))
			lg.msgERR(fmt.Sprintf(
				"exchange: config hash failed on message from: %+v",m.SrcHost))
			continue}
		
		//handle messages about connected unix socket clients
		if e.msg_handler_cluster_client_disconnect(&m) {continue}
		if e.msg_handler_cluster_client(&m) {continue}
		if e.msg_handler_cluster_ask_about_client_node(&m) {continue}
		//forward client message to "__master__"
		if e.msg_handle_client_msg_to_master(&m) {continue}
		
		//pass Logger messages
		if e.msg_handler_forward_to_logger(&m) {continue}
			
		//pass Brain messages
		if e.msg_handler_forward_to_brain(&m) {continue}
		
		//update heartbeat values from heartbeat messages
		if m.validate_Heartbeat() {
			if config.CheckIfNodeExists(&m.SrcHost){
				timeCopy := m.Time
				e.heartbeatLastMsg[m.SrcHost]=&timeCopy}
			continue}
		lg.msg_debug(1, fmt.Sprintf("exchange received message which failed all validation functions: %+v",m))}}

func (e *Exchange)placeholderStupidVariableNotUsedError(){
	lg.msg("exchange started")}

func (m *message)validate_Heartbeat() bool {
	return (
		m.SrcMod == msgModExchnHeartbeat &&
		m.DestMod == msgModExchnHeartbeat &&
		m.RpcFunc == rpcHeartbeat )}

func (e *Exchange)dumpAllConnectedHosts(){
	e.rwmux.RLock()
	fmt.Println(e.outgoing)
	fmt.Println(e.incoming)
	e.rwmux.RUnlock()
	}

func (e *Exchange)heartbeatSender(){
	var m message
	var t time.Time
	lg.msg_debug(3, "exchange launched heartbeatSender()")
	for{
		if e.killExchange { //ugly solution
			return}
		t = time.Now()
		m = message{
			SrcHost: config.MyHostname,
			DestHost: "__everyone__",
			SrcMod: msgModExchnHeartbeat,
			DestMod: msgModExchnHeartbeat,
			RpcFunc: rpcHeartbeat,
			Time: t,
			Argc: 1,
			Argv: []string{"heartbeat"},
			}
		e.loc_ex <- m
		time.Sleep(time.Millisecond * time.Duration(config.HeartbeatInterval))}}

func (e *Exchange)printHeartbeatStats(){
	if config.DebugHeartbeat {
		fmt.Printf("\n === Heartbeat info per node === \n")
		for k,v:= range e.heartbeatLastMsg{
			fmt.Printf("NODE: %s last heartbeat message %s\n", k, v.String())
			dt := time.Now().Sub(*v)
			fmt.Printf("NODE: %s last heartbeat delta %s\n", k, dt.String())}
		fmt.Printf(" === END of Heartbeat info === \n\n")}}

func (e *Exchange)KillExchange(){
	var brnOp, logOp, locOp bool = false, false, false
	var brnK, logK bool = false, false
	if e.killExchange {
		return}
	fmt.Println("KillExchange() starts")
	e.killExchange=true
	time.Sleep(time.Millisecond * time.Duration(100))

	close(e.recQueue)
	for{
		select{
		case _,brnOp = <-e.brn_ex:
			if ! brnOp && ! brnK {
				brnK=true
				close(e.ex_brn)}
		case _,logOp = <-e.log_ex:
			if ! logOp && ! logK {
				logK=true
				close(e.ex_log)}
		//case _,locOp = <-e.loc_ex:
		}
		if ! ( brnOp || logOp || locOp ) {
			break
		}}}

func (e *Exchange)GetHeartbeat()(map[string]*time.Time){
	return e.heartbeatLastMsg}

func (m *message)verifyMessageConfigHash() bool {
	if config.GetField_string("ConfFileHash") == m.ConfHash {
		return true
	}else{
		return false}}

//func (e *Exchange)verifyMessageConfigHash(m *message) bool {
//	if ! e.confHashCheck {
//		return true }
//	if e.confFileHash == m.ConfHash {
//		return true
//	}else{
//		return false }}

func (e *Exchange)markMessageWithConfigHash(m *message){
	if e.confHashCheck {
		m.ConfHash = e.confFileHash }}

func (e *Exchange)msg_handler_forward_to_brain(m *message) bool {
	if	m.SrcHost != config.MyHostname &&
		m.SrcMod == msgModBrain &&
		m.DestMod == msgModBrain {
		
		if config.DebugNetwork {
			fmt.Printf("DEBUG SORTER passed to brain %+v\n", m)}
		e.ex_brn <- *m;
		return true}
	return false}

func (e *Exchange)msg_handler_forward_to_logger(m *message) bool {
	if	m.logger_message_validate() {
		if config.DebugNetwork {
			fmt.Printf("DEBUG SORTER passed to logger %+v\n", m)}
		e.ex_log <- *m;
		return true}
	return false}

func (ec *eclient)sendUsockClientID(id uint){
		var t time.Time = time.Now()
		m := message{
			SrcHost:	config._MyHostname(),
			DestHost:	"__new_client__",
			SrcMod:		msgModExchn,
			DestMod:	msgModClient,
			RpcFunc:	exchangeSendClientID,
			Time:		t,
			Argc:		1,
			Argv:		[]string{ec.hostname},
			custom1:	id,
			}
		ec.outgoing <- m}

func (e *Exchange)notifyClusterAboutClient(id string, hostname string){
		var t time.Time = time.Now()
		m := message{
			SrcHost:	hostname,
			DestHost:	"__everyone__",
			SrcMod:		msgModExchn,
			DestMod:	msgModExchn,
			RpcFunc:	exchangeNotifyAboutClient,
			Time:		t,
			Argc:		2,
			Argv:		[]string{id,hostname},
			}
		ec.outgoing <- m}

func (e *Exchange)msg_handler_cluster_client(m *message) bool {
	if	m.RpcFunc == exchangeNotifyAboutClient &&
		config.CheckIfNodeExists(&m.SrcHost) &&
		m.Argc == 2 &&
		len(m.Argv) == 2 {
		
		//maybe add mutex
		e.clientMap[m.Argv[0]] = m.Argv[1]
		return true}
	return false}


func (e *Exchange)notifyClusterAboutClientDisconnect(id string){
		var t time.Time = time.Now()
		var l_hostname string = config._MyHostname()
		m := message{
			SrcHost:	l_hostname,
			DestHost:	"__everyone__",
			SrcMod:		msgModExchn,
			DestMod:	msgModExchn,
			RpcFunc:	exchangeNotifyClientDisconnect,
			Time:		t,
			Argc:		2,
			Argv:		[]string{id,l_hostname},
			}
		e.loc_ex <- m}

func (e *Exchange)msg_handler_cluster_client_disconnect(m *message) bool {
	if	m.RpcFunc == exchangeNotifyClientDisconnect &&
		config.CheckIfNodeExists(&m.SrcHost) &&
		m.Argc == 2 &&
		len(m.Argv) == 2 {
		
		//maybe add mutex
		delete(e.clientMap, m.Argv[0])
		return true}
	return false}


func (e *Exchange)askClusterAboutClientNode(id string){
		var t time.Time = time.Now()
		var l_hostname string = config._MyHostname()
		m := message{
			SrcHost:	l_hostname,
			DestHost:	"__everyone__",
			SrcMod:		msgModExchn,
			DestMod:	msgModExchn,
			RpcFunc:	exchangeAskAboutClientNode,
			Time:		t,
			Argc:		1,
			Argv:		[]string{id},
			}
		e.loc_ex <- m}

func (e *Exchange)msg_handler_cluster_ask_about_client_node(m *message) bool {
	var clientNodeName string
	var ok bool
	if	m.RpcFunc == exchangeAskAboutClientNode &&
		config.CheckIfNodeExists(&m.SrcHost) &&
		m.Argc == 2 &&
		len(m.Argv) == 2 {
		
		//maybe add mutex
		if clientNodeName, ok = e.clientMap[m.Argv[0]]; ok {
			//client found in map, sending reply
			e.notifyClusterAboutClient(m.Argv[0], clientNodeName)}
		return true}
	return false}

func (e *Exchange)msg_handle_client_msg_to_master(m *message) bool {
	var masterNode string
	if	m.SrcMod == msgModClient &&
		m.DestHost == "__master__"{
		if temp := b.getMasterNodeName(); temp == nil {
			// cluster has no master
			lg.msg_debug(2, "couldn't forward message to master, cluster has no master")
		}else{
			// cluster has a master
			masterNode = *temp}
			
		if config._MyHostname() == masterNode {
			e.ex_brn <- *m
			return true}
		e.rwmux.RLock()
		if masterNode != config._MyHostname() &&
			e.outgoing[masterNode] != nil {
			if config.DebugNetwork {
				fmt.Printf("DEBUG forwarder pushing to %s  %+v\n",
					masterNode, m)}
			e.outgoing[masterNode].outgoing <- *m }
			e.rwmux.RUnlock()
			return true }
	return false}
