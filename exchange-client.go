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
import "encoding/json"
import "fmt"

type eclient struct{
	originLocal	bool
	usock		bool
	hostname	string
	outgoing	chan message
	incoming	chan message
	conn		net.Conn
	exch		*Exchange
	}

func (ec *eclient)listen(){
	fmt.Printf("conn listener started for %+v\n", ec.conn)
	d := json.NewDecoder(ec.conn)
	var m message
	var err error
	for{
		err = d.Decode(&m)
		if config.DebugNetwork {
			fmt.Printf("conn Listener received %+v\n", m)}
		if err == nil{
			if ec.conn != nil{
				ec.incoming <- m}
		}else{
			break}}
	ec.conn.Close()
	if ec.conn != nil{
		ec.conn = nil}
	//delete itself from the map if running usock
	if ec.usock {
		ec.exch.notifyClusterAboutClientDisconnect(ec.hostname)
		e.rwmuxUSock.Lock()
		//delete because map usock keys are not reused
		delete(ec.exch.outgoing, ec.hostname)
		delete(ec.exch.cliLogTap, ec.hostname)
		e.rwmuxUSock.Unlock()
		}
	ec = nil
	lg.err("eclient Decoder ", err)}

func (ec *eclient)forward(){
	var data message
	var err error
	var cleanup_loop bool = true
	var chanOpen bool = false
	enc := json.NewEncoder(ec.conn)
	fmt.Printf("conn Forwarder started for %s\n", ec.hostname)
	for{
		data, chanOpen = <-ec.outgoing
		if ! chanOpen {
			lg.msg_debug(2, fmt.Sprintf("%+v outgoing channel closed",ec))
			break}
		if config.DebugNetwork {
			fmt.Printf("conn Forwarder to %s received %+v\n", ec.hostname,  data)}
		err = enc.Encode(data)
		if err != nil{
			break}}
	if ! ec.usock {
		ec.conn.Close()
		}
	if ec.conn != nil{
		ec.conn = nil}
	//lock exchange writing mutex
	// THIS MUTEX CAUSES ISSUES WITH LOCKUP ON DISCONNECTING CLIENT
	//ec.exch.rwmux.Lock()
	//if ! ec.usock {
	//	ec.exch.outgoing[ec.hostname]=nil
	//}else{
	//	ec.exch.outgoing[ec.hostname]=nil
	ec.exch.outgoing[ec.hostname]=nil
	//unlock exchange writing mutex 
	//ec.exch.rwmux.Unlock()
	
	//read all queued messages 
	for cleanup_loop {
		data,cleanup_loop = <-ec.outgoing
		//fmt.Println("CLEANUP LOOP ",data)
		}
	ec = nil
	lg.err("eclient forwarder serializer ", err)}
