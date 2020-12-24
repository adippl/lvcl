/*
 *  lvcl is a simple program clustering libvirt servers
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


type Exchange struct{
	myHostname	string
	nodeList	*[]Node
	dialers		map[string]*net.Conn
	listenTCP	net.Listener
	listenUnix	net.Listener

	brainIN		chan<- message
	brainOUT	<-chan message
	loggerIN	chan<- message
	loggerOUT	<-chan message 
	}

func (e Exchange)initListen(){
	var err error
	e.listenTCP, err = net.Listen("tcp", ":" + config.TCPport) 
	if err != nil {
		fmt.Printf("ERR, %s\n",err)}
	
	e.listenUnix, err = net.Listen("unix", config.UnixSocket) 
	if err != nil {
		fmt.Printf("ERR, %s\n",err)}
	}

func (e Exchange)startConn(n *Node){
		if(n.Hostname == e.myHostname){
			return}
		c,err:=net.Dial("tcp",n.NodeAddress)
		if(err!=nil){
			fmt.Printf("ERR %s\n",err)
			return}
		e.dialers[n.Hostname]=&c}

	/* TODO replace with functions independently establishing connections */
func (e Exchange)initConnections(){
	for _,n:= range *e.nodeList{
		go e.startConn(&n)
		}}

//func (e Exchange)New(){

