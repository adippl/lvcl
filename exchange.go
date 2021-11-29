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
import "strconv"

var e *Exchange

type Exchange struct{
	nodeList	*[]Node
	outgoing		map[string]*eclient
	incoming		map[string]*eclient
	usocks			map[string]*eclient
	listenTCP		net.Listener
	listenUnix		net.Listener
	clientMap		map[string]string
	//list of usock clients listening to cluster logger
	cliLogTap		map[string]int
	
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
		cliLogTap:		make(map[string]int),
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
	go e.configEpochSender()
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
		time.Sleep(time.Millisecond *
			time.Duration(config.ClusterTickInterval))}}

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
	var ec *eclient
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
		
		ec = &eclient{
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
		
		// logger messages to listening clients
		if e.msg_handle_forward_logger_to_client_tap(m) {continue}
		
		// forward brain status message to client
		if e.msg_handle_clientPrintTextStatus(m) {continue}
		
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
		
		//handle client reguest for logger message
		if e.msg_handle_client_logger_listen_connect(&m) {continue}
		//handle client reguest for cluster status info
		if e.msg_handle_clientAskAboutStatus(&m) {continue}
		
		//handle client's request to change resource state
		if e.msg_handle_msgModConfig(&m) {continue}
		
		//forward client message to "__master__"
		if e.msg_handle_client_msg_to_master(&m) {continue}
		
		//config message
		if e.msg_handle_confNotifAboutEpoch(&m) {continue}
		if e.msg_handle_confNotifAboutEpochUpdateAsk(&m) {continue}
		if e.msg_handle_confNotifAboutEpochUpdate(&m) {continue}
		
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

func (e *Exchange)configEpochSender(){
	var m message
	var t time.Time
	lg.msg_debug(3, "exchange launched configEpochSender()")
	for{
		if e.killExchange { //ugly solution
			return}
		t = time.Now()
		
		m = message{
			SrcHost: config.MyHostname,
			DestHost: "__everyone__",
			SrcMod: msgModConfig,
			DestMod: msgModConfig,
			RpcFunc: confNotifAboutEpoch,
			Time: t,
			Argc: 1,
			Argv: []string{"epoch"},
			Cint: config.GetEpoch(),
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

func (e *Exchange)sendLoggerStartForwardToClient(){
	e.ex_log <- message{
		SrcMod: msgModExchn,
		DestMod: msgModLoggr,
		RpcFunc: loggerForwardMessageToClient}}

func (e *Exchange)sendLoggerStartForwardToClientStop(){
	e.ex_log <- message{
		SrcMod: msgModExchn,
		DestMod: msgModLoggr,
		RpcFunc: loggerForwardMessageToClientStop}}

func (e *Exchange)msg_handle_forward_logger_to_client_tap(m *message) bool {
	var mod_m message
	if	m.SrcMod == msgModLoggr &&
		m.DestMod == msgModClient {
		
		e.rwmuxUSock.RLock()
		if len(e.cliLogTap) == 0 {
			e.rwmuxUSock.RUnlock()
			// TODO DON'T LOG WITH A LOGGER DURING LOGGER STUFF...
			//go lg.msg_debug(2, "no unix socket client listening, stopping forwarding log messages to clients")
			// TODO notify logger with message
			//lg.forwardToCli = false
			e.sendLoggerStartForwardToClientStop()
			return true
		}else{
			e.rwmuxUSock.RUnlock()}
		
		mod_m = *m
		mod_m.RpcFunc = clientPrintTextLogger
		mod_m.DestMod = msgModClient
		mod_m.SrcMod = msgModLoggr
		
		if config.DebugNetwork {
			fmt.Println("DEBUG tap forwarding logger message to client", 
				mod_m)}
		e.rwmuxUSock.RLock()
		for k,_ := range e.cliLogTap {
			mod_m.DestHost = k
			if config.DebugNetwork {
				fmt.Printf("DEBUG SORTER passed to client %s %+v\n", k, m)}
			e.outgoing[k].outgoing <- mod_m;}
		e.rwmuxUSock.RUnlock()
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
		e.loc_ex <- m}

func (e *Exchange)msg_handler_cluster_client(m *message) bool {
	if	m.RpcFunc == exchangeNotifyAboutClient &&
		config.CheckIfNodeExists(&m.SrcHost) &&
		m.Argc == 2 &&
		len(m.Argv) == 2 {
		
		//maybe add mutex
		e.rwmuxUSock.Lock()
		e.clientMap[m.Argv[0]] = m.Argv[1]
		e.rwmuxUSock.Unlock()
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
		m.DestHost == "__master__" {
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

func (e *Exchange)msg_handle_client_logger_listen_connect(m *message) bool {
	var loglevel int
	var err error = nil
	if	m.SrcMod == msgModClient &&
		m.RpcFunc == clientListenToClusterLogger &&
		m.DestHost == "__any__" {
		
		//fmt.Println("-=-=-=- DEBUG received client's ask for logger data")
		loglevel, err = strconv.Atoi(m.Argv[0])
		if err != nil {
			lg.err("clientListenToClusterLogger wrong m.Argv[0] ", err)
			// TODO client should commit suicide
			return false}
		e.rwmuxUSock.Lock()
		e.cliLogTap[m.SrcHost] = loglevel
		fmt.Println(e.cliLogTap)
		for k, v := range e.cliLogTap {
		    fmt.Println(k, "value is", v)}
		//TODO SEND MESSAGE INSTEAD OF DIRECT WRITE
		//lg.forwardToCli= true
		e.sendLoggerStartForwardToClient()
		e.rwmuxUSock.Unlock()
		return true}
	return false}

func (e *Exchange)msg_handle_clientAskAboutStatus(m *message) bool{
	if	m.RpcFunc == clientAskAboutStatus &&
		m.DestMod == msgModBrain {
		
		e.ex_brn <- *m
		return true}
	return false}

func (e *Exchange)msg_handle_clientPrintTextStatus(m *message) bool {
	var mod_m message
	if	m.RpcFunc == clientPrintTextStatus &&
		m.DestMod == msgModClient &&
		m.SrcMod == msgModBrain {
		
		if config.DebugNetwork {
			fmt.Println(
				"DEBUG exchange forwards cli status from brain to client", 
				mod_m)}
		e.rwmux.RLock()
		e.outgoing[m.DestHost].outgoing <- *m
			if config.DebugNetwork {
				fmt.Printf(
					"DEBUG SORTER passed to client %s %+v\n", m.DestHost, m)}
		e.rwmux.RUnlock()
		return true}
	return false}

func (e *Exchange)replyToUsock(m *message) {
	e.rwmuxUSock.RLock()
	e.outgoing[m.DestHost].outgoing <- *m
	e.rwmuxUSock.RUnlock()
	}
	

func (e *Exchange)msg_handle_msgModConfig(m *message) bool {
	var res *Cluster_resource = nil
	var newState int
	var reply message
	
	if	m.RpcFunc == clientAskResStateChange &&
		m.SrcMod == msgModClient &&
		m.DestMod == msgModConfig {
		
		//prepare reply message
		reply.SrcMod = msgModConfig
		reply.DestMod = msgModClient
		reply.DestHost = m.SrcHost
		reply.RpcFunc = clientAskResStateChangeReply
		reply.Argv = m.Argv
		
		if validateStateFlag(&m.Argv[1]) == false {
			//message arrived with wrong 
			reply.Cint=11
			reply.Argv = append(reply.Argv, "error wrong state requested")
			lg.msgERR("error wrong state requested")
			e.replyToUsock(&reply)
			return true}
		
		switch m.Argv[1] {
		case "on":
			newState = resource_state_running
		case "off":
			newState = resource_state_stopped
		case "pause":
			newState = resource_state_paused
		case "reboot":
			newState = resource_state_reboot
		case "nuke":
			newState = resource_state_nuked
		default:
			//reply with error
			reply.Cint=12
			reply.Argv = append(reply.Argv, "error wrong state requested 2")
			lg.msgERR("error wrong state requested 2")
			e.replyToUsock(&reply)
			return true}
		
		
		res = config.GetCluster_resourcebyName_RW(&m.Argv[0])
		if res == nil {
			// resource doesn't exists
			reply.Cint=13
			reply.Argv = append(reply.Argv, "couldn't find resource")
			lg.msgERR("couldn't find resource")
			e.replyToUsock(&reply)
			return true}
		//mutex
		config.rwmux.Lock()
		res.State = newState
		config.rwmux.Unlock()
		//end of mutex
		config.IncEpoch() //TODO this function uses the mutex again
		lg.msg_debug(2, fmt.Sprintf("resource %s changing state to %s",
			res.Name, res.StateString()))
		//positive reply
		reply.Cint=0
		reply.Argv = append(reply.Argv, "resource changed state")
		e.replyToUsock(&reply)
		return true}
	return false}

func (e *Exchange)msg_handle_confNotifAboutEpoch(m *message) bool {
	var bk_epo int = -1
	if	m.RpcFunc == confNotifAboutEpoch &&
		m.SrcMod == msgModConfig &&
		m.DestMod == msgModConfig &&
		m.SrcHost != config._MyHostname() {
			
		if config.isTheirEpochBehind(m.Cint) {
			// TODO respond by sending them our config
			m.DestHost = m.SrcHost
			m.SrcHost = config._MyHostname()
			m.RpcFunc = confNotifAboutEpochUpdateAsk
			m.Argv[0] = "requesting config from host with higher epoch"
			bk_epo = m.Cint
			m.Cint = config.GetEpoch()
			e.loc_ex <- *m
			lg.msg_debug(2, fmt.Sprintf(
				"found node with higher epoch %s (our %d theirs %d)",
				m.DestHost, config.GetEpoch(), bk_epo))}
			return true}
	return false}


func (e *Exchange)msg_handle_confNotifAboutEpochUpdateAsk(m *message) bool {
	if	m.RpcFunc == confNotifAboutEpochUpdateAsk &&
		m.SrcMod == msgModConfig &&
		m.DestMod == msgModConfig &&
		m.SrcHost != config._MyHostname() {
			
		// TODO maybe check if for epoch
		//if config.isTheirEpochBehind(m.Cint) {
		if true {
			// TODO respond by sending them our config
			m.DestHost = m.SrcHost
			m.SrcHost = config._MyHostname()
			m.RpcFunc = confNotifAboutEpochUpdate
			m.Argv[0] = "sending config with newer epoch"
			m.Cint = config.GetEpoch()
			config.rwmux.RLock()
			m.Res = config.Resources
			config.rwmux.RUnlock()
			e.loc_ex <- *m
			fmt.Println("ALKWDJLAKWJDLAKJWDLAWD")
			lg.msg_debug(2, fmt.Sprintf(
				"sending config to node %s (epoch %d)",
				m.DestHost, m.Cint))}
			return true}
	return false}

func (e *Exchange)msg_handle_confNotifAboutEpochUpdate(m *message) bool {
	if	m.RpcFunc == confNotifAboutEpochUpdate &&
		m.SrcMod == msgModConfig &&
		m.DestMod == msgModConfig &&
		m.DestHost == config._MyHostname() {
			
		// TODO maybe check if for epoch
		// check if message arrived with config from higher epoch
		//if config.isTheirEpochBehind(m.Cint) {
		if true {
			config.rwmux.Lock()
			config.Resources = m.Res
			config.Epoch = m.Cint
			config.rwmux.Unlock()
			lg.msg_debug(2, fmt.Sprintf(
				"received config from node %s with epoch %d ",
				m.SrcHost, m.Cint))}
			return true}
	return false}

