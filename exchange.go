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

var e *Exchange

type Exchange struct{
	nodeList	*[]Node
	outgoing		map[string]*eclient
	incoming		map[string]*eclient
	listenTCP	net.Listener
	listenUnix	net.Listener
	
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
		killExchange:	false,
		recQueue:		make(chan message),
		loc_ex:			make(chan message),
		brn_ex:			a_brn_ex,
		ex_brn:			a_ex_brn,
		log_ex:			a_log_ex,
		ex_log:			a_ex_log,
		rwmux:			sync.RWMutex{},
		confFileHash:	config.GetField_string("ConfFileHash"),
		confHashCheck:	config.GetField_bool("ConfHashCheck"),
		}
	
	go e.initListen()
	go e.reconnectLoop()
	go e.forwarder()
	go e.sorter()
	go e.heartbeatSender()
	go e.updateNodeHealthDelta()
	return &e}

func (e *Exchange)updateNodeHealthDelta(){
	if e.killExchange {
		return}
	for{
		// creating new map is much easier than searching for missing nodes
		e.heartbeatDelta=make(map[string]*time.Duration)
		for k,v := range e.heartbeatLastMsg {
			dt := time.Now().Sub(*v)
			e.heartbeatDelta[k]=&dt}
		time.Sleep(time.Millisecond * time.Duration(config.ClusterTickInterval))}}
//		time.Sleep(time.Millisecond * time.Duration(config.NodeHealthCheckInterval))}}

//func (e *Exchange)tcpHandleListen(c net.Conn) *eclient{ //TODO move into initListen
//	eclient := &eclient{
//		incoming:	e.recQueue,
//		conn:		c,}
//	go eclient.listen()
//	return eclient}

func (e *Exchange)initListen(){
	var err error
	e.listenTCP, err = net.Listen("tcp", ":" + config.TCPport)
	if err != nil {
		lg.msg(fmt.Sprintf("ERR, net.Listen , %s",err))}
	for{
		if e.killExchange { //ugly solution
			return}
		conn,err := e.listenTCP.Accept()
		if err != nil {
			lg.msg(fmt.Sprintf("ERR, net.Listener.Accept() , %s",err))}
		
		raddr := conn.RemoteAddr().String()
		laddr := conn.LocalAddr().String()
		lg.msg(fmt.Sprintf("info, %s connected to node %s ", raddr, laddr))
		
		ec := &eclient{
			hostname:	conn.RemoteAddr().String(),
			incoming:	e.recQueue,
			conn:		conn,}
		go ec.listen()}}

func (e *Exchange)reconnectLoop(){
	for{
		if e.killExchange { //ugly solution
			return}
		e.rwmux.RLock()
		for _,n := range *e.nodeList{
			if e.outgoing[n.Hostname] == nil && n.Hostname != config.MyHostname {
				lg.msg_debug(fmt.Sprintf("attempting to recconect to host %s",n.Hostname),1)
				go e.startConn(n)}}
		e.rwmux.RUnlock()
		time.Sleep(time.Millisecond * time.Duration(config.ReconnectLoopDelay))}}


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

func (e *Exchange)initListenUnix(){
	var err error
	e.listenUnix, err = net.Listen("unix", config.UnixSocket)
	if err != nil {
		lg.msg(fmt.Sprintf("ERR, net.Listen %s",err))}}

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
			fmt.Printf("\nbrnOk=%b logOk=%b locOk=%b\n", brnOk, logOk, locOk)}
		
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
					//fmt.Printf("DEBUG forwarder pushing to %s  %+v\n", n.Hostname, m)
					if config.DebugNetwork {
						fmt.Printf("DEBUG forwarder pushing to %s  %+v\n", n.Hostname, m)}
					////making sure one more time, (it could've changed during debug write to console)
					//if e.outgoing[n.Hostname] != nil {
					//	e.outgoing[n.Hostname].outgoing <- *m }}
					e.outgoing[n.Hostname].outgoing <- *m }
					e.rwmux.RUnlock()}}}}

func (m *message)_check_pass_message_to_logger() bool {
	return 	( m.SrcHost != config._MyHostname() &&
				m.SrcMod == msgModLoggr &&
				m.DestMod == msgModLoggr &&
				m.RpcFunc == 1 &&
				m.Argc == 1 )}

func (e *Exchange)sorter(){
	var m message
	for{
		if e.killExchange { //ugly solution
			return}
		m = message{}
		m = <-e.recQueue
		if config.DebugNetwork {
			fmt.Printf("DEBUG SORTER received %+v\n", m)}
		
		// check if message comes with  
		if ! e.verifyMessageConfigHash(&m) {
			lg.msg_debug(fmt.Sprintf("exchange: validation config hash: %+v",m),1)
			lg.msgERR(fmt.Sprintf("exchange: config hash failed on message from: %+v",m.SrcHost))
			continue}
		
		//pass Logger messages
		if	m.logger_message_validate() {
			if config.DebugNetwork {
				fmt.Printf("DEBUG SORTER passed to logger %+v\n", m)}
			e.ex_log <- m;
			continue}
			
		if	m.validate_Brain() {
			if config.DebugNetwork {
				fmt.Printf("DEBUG SORTER passed to brain %+v\n", m)}
			e.ex_brn <- m;
			continue}
		
		//update heartbeat values from heartbeat messages
		if m.validate_Heartbeat() {
			if config.CheckIfNodeExists(&m.SrcHost){
				timeCopy := m.Time
				e.heartbeatLastMsg[m.SrcHost]=&timeCopy}
			continue}
		lg.msg_debug(fmt.Sprintf("exchange received message which failed all validation functions: %+v",m),1)}}

func (e *Exchange)placeholderStupidVariableNotUsedError(){
	lg.msg("exchange started")}

func (m *message)validate_Heartbeat() bool {
	return (
		m.SrcMod == msgModExchnHeartbeat &&
		m.DestMod == msgModExchnHeartbeat &&
		m.RpcFunc == rpcHeartbeat )}

func (m *message)validate_Brain() bool {
	return (
	m.SrcHost != config.MyHostname &&
	m.SrcMod == msgModBrain &&
	m.DestMod == msgModBrain )}

func (e *Exchange)dumpAllConnectedHosts(){
	e.rwmux.RLock()
	fmt.Println(e.outgoing)
	fmt.Println(e.incoming)
	e.rwmux.RUnlock()
	}

func (e *Exchange)heartbeatSender(){
	var m message
	var t time.Time
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
	e.killExchange=true}

func (e *Exchange)GetHeartbeat()(map[string]*time.Time){
	return e.heartbeatLastMsg}

func (m *message)verifyMessageConfigHash() bool {
	if config.GetField_string("ConfFileHash") == m.ConfHash {
		return true
	}else{
		return false}}

func (e *Exchange)verifyMessageConfigHash(m *message) bool {
	if ! e.confHashCheck {
		return true }
	if e.confFileHash == m.ConfHash {
		return true
	}else{
		return false }}

func (e *Exchange)markMessageWithConfigHash(m *message){
	if e.confHashCheck {
		m.ConfHash = e.confFileHash }}
