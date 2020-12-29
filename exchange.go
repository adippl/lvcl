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
	myHostname	string
	nodeList	*[]Node
	dialed		map[string]*net.Conn
	dialers		map[string]*eclient
	listenTCP	net.Listener
	listenUnix	net.Listener
	
	
	recQueue	chan message
	brainIN		chan<- message
	loggerIN	chan<- message
	exIN		<-chan message
	}



func NewExchange(exIN <-chan message, bIN chan<- message, lIN chan<- message) *Exchange {
	e := Exchange{
		myHostname:	config.MyHostname,
		nodeList:	&config.Nodes,
		dialed:		make(map[string]*net.Conn),
		dialers:	make(map[string]*eclient),
		recQueue:	make(chan message, 33),
		brainIN:	bIN,
		loggerIN:	lIN,
		exIN:		exIN,
		}
	go e.initListen()
	go e.initConnections()
	return &e}

func (e *Exchange)tcpHandle(c net.Conn) *eclient{
	eclient := &eclient{
		incoming:	make(chan message),
		outgoing:	make(chan message),
		exchange:	e.recQueue,
		conn:		c,}
	eclient.listen()
	return eclient }

func (e *Exchange)initListen(){
	var err error
	e.listenTCP, err = net.Listen("tcp", ":" + config.TCPport)
	if err != nil {
		lg.msg(fmt.Sprintf("ERR, net.Listen , %s\n",err))}
	for {
		conn,err := e.listenTCP.Accept()
		if err != nil {
			lg.msg(fmt.Sprintf("ERR, net.Listener.Accept() , %s\n",err))}
		raddr := conn.RemoteAddr().String()
		laddr := conn.LocalAddr().String()
		lg.msg(fmt.Sprintf("info, %s connected to node %s \n", raddr, laddr))
		ec := e.tcpHandle(conn)
		e.dialers[raddr]=ec
		}
	}

func (e *Exchange)initListenUnix(){
	var err error
	e.listenUnix, err = net.Listen("unix", config.UnixSocket)
	if err != nil {
		lg.msg(fmt.Sprintf("ERR, net.Listen %s\n",err))}
	}

func (e *Exchange)startConn(n Node){
		if(n.Hostname == e.myHostname){
			return}
		c,err:=net.Dial("tcp",n.NodeAddress)
		lg.msg(fmt.Sprintf("ERR, dialing %s \"%s\"\n", n.Hostname, err))
		if(err!=nil){
			e.dialed[n.Hostname]=nil
			return}
		e.dialed[n.Hostname]=&c}

func (e *Exchange)reconnectLoop(){
	for{
		for _,n := range *e.nodeList{
			//fmt.Printf("[ %s ]\n",n)
			if e.dialed[n.Hostname] == nil{
				go e.startConn(n)}
				}
			time.Sleep(time.Millisecond * time.Duration(config.ReconnectLoopDelay))}}

func (e *Exchange)initConnections(){
	go e.reconnectLoop()
	for _,n:= range *e.nodeList{
		go e.startConn(n)}}
	

func (e *Exchange)forwarder(){
	for{
		for m := range e.exIN{
			m.dump()}
		}}

//func dateToTime() time.Time{
