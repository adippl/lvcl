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
import "bufio"

type eclient struct{
	originLocal	bool
	incoming	chan message
	outgoing	chan message
	reader		*bufio.Reader
	writer		*bufio.Writer
	conn		net.Conn
	connection	*eclient
	}

//func  (ec *eclient) listen(){
	
func (ec *eclient) Read() {
	for {
		line, err := ec.reader.ReadString('\n')
		if err == nil {
			if ec.connection != nil {
				ec.connection.outgoing <- line
			}
			fmt.Println(line)
		} else {
			break } }

	ec.conn.Close()
	delete(alleecs, ec)
	if ec.connection != nil {
		ec.connection.connection = nil }
	ec = nil }

func (ec *eclient) Write() {
	for data := range ec.outgoing {
		ec.writer.WriteString(data)
		ec.writer.Flush() } }
