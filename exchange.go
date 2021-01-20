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
	dialed		map[string]*eclient
	dialers		map[string]*eclient
	listenTCP	net.Listener
	listenUnix	net.Listener
	
	lastHeartbeat		map[string]*time.Time
	heartbeatHandler chan message
	
	recQueue	chan message
	brainIN		chan<- message
	loggerIN	chan<- message
	exIN		chan message
	}



func NewExchange(exIN chan message, bIN chan<- message, lIN chan<- message) *Exchange {
	e := Exchange{
		//myHostname:	config.MyHostname,
		nodeList:	&config.Nodes,
		dialed:		make(map[string]*eclient),
		dialers:	make(map[string]*eclient),
		lastHeartbeat:	make(map[string]*time.Time),
		heartbeatHandler:	make(chan message),
		recQueue:	make(chan message),
		brainIN:	bIN,
		loggerIN:	lIN,
		exIN:		exIN,
		}
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
		e.dialers[raddr]=ec
		}
	}

func (e *Exchange)startConn(n Node){	//TODO connect to eclient
	if(n.Hostname == config.MyHostname){
		return}
	c,err := net.Dial("tcp",n.NodeAddress)
	if(err!=nil){
		lg.msg(fmt.Sprintf("ERR, dialing %s error: \"%s\"", n.Hostname, err))
		e.dialed[n.Hostname]=nil //just to be sure
		return}
	ec := eclient{
		hostname:		n.Hostname,
		originLocal:	true,
		outgoing:		make(chan message),
		conn:			c,
		exch:			e,
		}
	go ec.forward();
	e.dialed[n.Hostname]=&ec}

func (e *Exchange)initListenUnix(){
	var err error
	e.listenUnix, err = net.Listen("unix", config.UnixSocket)
	if err != nil {
		lg.msg(fmt.Sprintf("ERR, net.Listen %s",err))}
	}

func (e *Exchange)reconnectLoop(){
	for{
		for _,n := range *e.nodeList{
			//fmt.Printf("[ %s ]\n",n)
			if e.dialed[n.Hostname] == nil{
				go e.startConn(n)}
				}
			time.Sleep(time.Millisecond * time.Duration(config.ReconnectLoopDelay))}}

func (e *Exchange)forwarder(){
	var debug bool = false
	var m message
	if config.DEbugLogAllAtExchange {
		debug=true}
	debug=true
		
	//fmt.Println("DEBIG MAIN forwarder starts")
	for{
		m = <-e.exIN
		fmt.Printf("exchange forwarder received %+v\n",m)
		// DEBUG outgoing messages
		if debug && m.SrcMod != msgModLoggr && m.DestMod != msgModLoggr && m.SrcHost == config.MyHostname {
			lg.DEBUGmessage(&m)}
		// outgoing send to all
//		if	m.SrcHost == config.MyHostname && m.DestHost == "__everyone__" &&
//			m.SrcMod == msgModLoggr && m.DestMod == msgModLoggr &&
//			m.SrcMod == msgModExchnHeartbeat && m.DestMod == msgModExchnHeartbeat {
		
		
		//forward to everyone else
		if	m.SrcHost == config.MyHostname && m.DestHost == "__everyone__" {
			for _,n := range config.Nodes{
				if e.dialed[n.Hostname] != nil && n.Hostname != config.MyHostname {
					fmt.Printf("DEBUG forwarder pushing to %s  %+v\n", n.Hostname, m)
					e.dialed[n.Hostname].outgoing <- m
					}}}
					}}

func (e *Exchange)sorter(){
	var m message
	for{
		m = <-e.recQueue
		fmt.Printf("DEBUG SORTER received u%+v\n", m)}}


func (e *Exchange)placeholderStupidVariableNotUsedError(){
	lg.msg("exchange started")}

func (e *Exchange)dumpAllConnectedHosts(){
	fmt.Println("outgoing connections")
	fmt.Println(e.dialed)
	fmt.Println("incoming connenctions")
	fmt.Println(e.dialers)}

func (e *Exchange)heartbeatSender(){
	var m message
	var t time.Time
	for{
		t = time.Now()
		m = message{ SrcHost: config.MyHostname,
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

