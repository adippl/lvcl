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



type Exchange struct{
	//myHostname	string
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
	exIN		chan message
	
	killExchange	bool // very ugly solution
	}



func NewExchange(exIN chan message, bIN chan<- message, lIN chan<- message) *Exchange {
	e := Exchange{
		//myHostname:	config.MyHostname,
		nodeList:	&config.Nodes,
		outgoing:		make(map[string]*eclient),
		incoming:		make(map[string]*eclient),
		heartbeatLastMsg:	make(map[string]*time.Time),
		heartbeatDelta:	make(map[string]*time.Duration),
		killExchange:	false,
		recQueue:	make(chan message),
		brainIN:	bIN,
		loggerIN:	lIN,
		exIN:		exIN, }
	
	go e.initListen()
	go e.reconnectLoop()
	go e.forwarder()
	go e.sorter()
	go e.heartbeatSender()
	return &e}

func (e *Exchange)tcpHandleListen(c net.Conn) *eclient{ //TODO move into initListen
	eclient := &eclient{
		incoming:	e.recQueue,
		conn:		c,
		//exch:		e,
		}
	go eclient.listen()
	return eclient }

func (e *Exchange)initListen(){
	var err error
	e.listenTCP, err = net.Listen("tcp", ":" + config.TCPport)
	if err != nil {
		lg.msg(fmt.Sprintf("ERR, net.Listen , %s",err))}
	for {
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
		go ec.listen()
		//e.dialers[raddr]=ec
		}
	}

func (e *Exchange)reconnectLoop(){
	for{
		if e.killExchange { //ugly solution
			return}
		for _,n := range *e.nodeList{
			//fmt.Printf("[ %s ]\n",n)
			if e.outgoing[n.Hostname] == nil{
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
		lg.msg(fmt.Sprintf("ERR, net.Listen %s",err))}
	}

func (e *Exchange)forwarder(){
	var debug bool = false
	var m message
	if config.DEbugLogAllAtExchange {
		debug=true}
	debug=true
		
	for{
		if e.killExchange { //ugly solution
			return}

		m = <-e.exIN
		// DEBUG outgoing messages
		if debug && m.SrcMod != msgModLoggr && m.DestMod != msgModLoggr && m.SrcHost == config.MyHostname {
			lg.DEBUGmessage(&m)
			}
		
		
		//forward to everyone else
		if	m.SrcHost == config.MyHostname && m.DestHost == "__everyone__" {
			for _,n := range config.Nodes{
				if n.Hostname != config.MyHostname && e.outgoing[n.Hostname] != nil {
					//fmt.Printf("\nDEBUG forwarder pushing to %s  %+v\n", n.Hostname, m)
					if e.outgoing[n.Hostname] != nil {
						e.outgoing[n.Hostname].outgoing <- m
						}
					}}}
					}}

func (e *Exchange)sorter(){
	var m message
	var dt time.Duration
	for{
		if e.killExchange { //ugly solution
			return}
		m = <-e.recQueue
		fmt.Printf("DEBUG SORTER received u%+v\n", m)
		
		//update heartbeat tab from heartbeat messages
		if m.SrcMod == msgModExchnHeartbeat && m.DestMod == msgModExchnHeartbeat && m.RpcFunc == rpcHeartbeat {
			if config.checkIfNodeExists(&m.SrcHost){
				//ADDING FALT 100ms to time delta to avoid negative values with small time differences (-10ms or -550µs)
				dt = time.Now().Sub(m.Time)
				//dt.Add(time.Millisecond * 100)
				dt = dt + (time.Millisecond * 100)
				e.heartbeatLastMsg[m.SrcHost]=&m.Time
				e.heartbeatDelta[m.SrcHost]=&dt
				}}}}


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
		e.exIN <- m
		fmt.Println("sending heartbeat")
		time.Sleep(time.Millisecond * time.Duration(config.HeartbeatInterval))}}

func (e *Exchange)printHeartbeatStats(){
	fmt.Printf("\n === Heartbeat info per node === \n")
	for k,v:= range e.heartbeatLastMsg{
		fmt.Printf("NODE: %s last heartbeat message %s\n", k, v.String())}
	for k,v:= range e.heartbeatDelta{
		fmt.Printf("NODE: %s last heartbeat delta %s\n", k, v.String())}
	fmt.Printf(" === END of Heartbeat info === \n\n")}

func (e *Exchange)KillExchange(){
	e.killExchange=true}

func (e *Exchange)GetHeartbeat()(map[string]*time.Time){
	return e.heartbeatLastMsg}

