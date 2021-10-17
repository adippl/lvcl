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
	brainIN		chan<- message
	loggerIN	chan<- message
	exchangeIN		chan message
	
	killExchange	bool // ugly solution
	}



func NewExchange(exIN chan message, bIN chan<- message, lIN chan<- message) *Exchange {
	e := Exchange{
		nodeList:	&config.Nodes,
		outgoing:		make(map[string]*eclient),
		incoming:		make(map[string]*eclient),
		heartbeatLastMsg:	make(map[string]*time.Time),
		heartbeatDelta:	make(map[string]*time.Duration),
		killExchange:	false,
		recQueue:	make(chan message),
		brainIN:	bIN,
		loggerIN:	lIN,
		exchangeIN:		exIN,
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

func (e *Exchange)tcpHandleListen(c net.Conn) *eclient{ //TODO move into initListen
	eclient := &eclient{
		incoming:	e.recQueue,
		conn:		c,}
	go eclient.listen()
	return eclient}

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
		for _,n := range *e.nodeList{
			if e.outgoing[n.Hostname] == nil && n.Hostname != config.MyHostname {
				lg.msg(fmt.Sprintf("DEBUG -=-=-=-=-=- attempting to recconect to host %s",n.Hostname))
				go e.startConn(n)}}
		time.Sleep(time.Millisecond * time.Duration(config.ReconnectLoopDelay))}}


func (e *Exchange)startConn(n Node){	//TODO connect to eclient
	if(n.Hostname == config.MyHostname){
		return}
	c,err := net.Dial("tcp",n.NodeAddress)
	if(err!=nil){
		lg.msg(fmt.Sprintf("ERR, dialing %s error: \"%s\"", n.Hostname, err))
		e.outgoing[n.Hostname]=nil //just to be sure
		return}
	ec := eclient{
		hostname:		n.Hostname,
		originLocal:	true,
		outgoing:		make(chan message),
		conn:			c,
		exch:			e,
		}
	go ec.forward();
	e.outgoing[n.Hostname]=&ec}

func (e *Exchange)initListenUnix(){
	var err error
	e.listenUnix, err = net.Listen("unix", config.UnixSocket)
	if err != nil {
		lg.msg(fmt.Sprintf("ERR, net.Listen %s",err))}}

func (e *Exchange)forwarder(){
	var m message
	for{
		if e.killExchange { //ugly solution
			return}
		
		m = <-e.exchangeIN
		if config.DebugNetwork {
			fmt.Printf("DEBUG forwarder recieved %+v\n", m)}
		if	m.SrcHost == config.MyHostname &&
			config.GetNodebyHostname(&m.DestHost) != nil &&
			e.outgoing[m.DestHost] != nil {
			e.outgoing[m.DestHost].outgoing <- m
		}else{
			fmt.Printf("DEBUG forwarder recieved INVALID message %+v\n", m)}
		
		//forward to everyone else
		if	m.SrcHost == config.MyHostname && m.DestHost == "__everyone__" {
			for _,n := range config.Nodes{
				if n.Hostname != config.MyHostname && e.outgoing[n.Hostname] != nil {
					//fmt.Printf("DEBUG forwarder pushing to %s  %+v\n", n.Hostname, m)
					if config.DebugNetwork {
						fmt.Printf("DEBUG forwarder pushing to %s  %+v\n", n.Hostname, m)}
					//making sure one more time, (it could've changed during debug write to console)
					if e.outgoing[n.Hostname] != nil {
						e.outgoing[n.Hostname].outgoing <- m }}}}}}

func (e *Exchange)sorter(){
	var m message
	for{
		if e.killExchange { //ugly solution
			return}
		m = <-e.recQueue
		if config.DebugNetwork {
			fmt.Printf("DEBUG SORTER received %+v\n", m)}
		
		//pass Logger messages
		if	m.SrcHost != config.MyHostname &&
			m.SrcMod == msgModLoggr &&
			m.DestMod == msgModLoggr &&
			m.RpcFunc == 1 &&
			m.Argc == 1 {
			
			if config.DebugNetwork {
				fmt.Printf("DEBUG SORTER passed to logger %+v\n", m)}
			e.loggerIN <- m;
			continue}
			
		if	m.SrcHost != config.MyHostname &&
			m.DestMod == msgModBrain{
				if m.ValidateMessageBrain(){
				e.brainIN <- m;
				}else{
					lg.msg(fmt.Sprintf("Brain message failed to validate: %+v",m))}
			continue}
		
		//update heartbeat values from heartbeat messages
		if m.SrcMod == msgModExchnHeartbeat && m.DestMod == msgModExchnHeartbeat && m.RpcFunc == rpcHeartbeat {
			if config.CheckIfNodeExists(&m.SrcHost){
				timeCopy := m.Time
				e.heartbeatLastMsg[m.SrcHost]=&timeCopy}}}}

func (e *Exchange)placeholderStupidVariableNotUsedError(){
	lg.msg("exchange started")}

func (e *Exchange)dumpAllConnectedHosts(){
	fmt.Println(e.outgoing)
	fmt.Println(e.incoming)
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
		e.exchangeIN <- m
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

