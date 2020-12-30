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

type eclient struct{
	originLocal	bool
	incoming	chan message
	outgoing	chan message
	exchange	chan message
	conn		net.Conn
	//exch		*Exchange
	}

func (ec *eclient)listen(){
	d := json.NewDecoder(ec.conn)
	var m message
	var err error
	for{
		err = d.Decode(&m)
		if err == nil{
			if ec.conn != nil{
				ec.exchange <- m}
		}else{
			lg.msgE("Decoder", err)
			break}}
	ec.conn.Close()
	if ec.conn != nil{
		ec.conn = nil}
	ec = nil}

func (ec *eclient)forward(){
	e := json.NewEncoder(ec.conn)
	for{
		for data := range ec.outgoing{
			err := e.Encode(data)
			if err != nil{
				lg.msgE("eclient deser", err)
				break}}}
	ec.conn.Close()
	if ec.conn != nil{
		ec.conn = nil}
	ec = nil}
