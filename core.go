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

import "fmt"
import "time"

func setup(){
	writeExampleConfig()
	confLoad()
	config.MyHostname="r210II-1" // faking hostname for testing
	brainIN:=make(chan message,10)
	loggerIN:=make(chan message,10)
	exchangeIN:=make(chan message,10)

	lg = NewLoger(loggerIN, exchangeIN)
	
	e := NewExchange(exchangeIN, brainIN, loggerIN)
	e.placeholderStupidVariableNotUsedError()
	m := Newmessage()
	str := "TESTTESTTEST"
	m.setStr(&str)
	exchangeIN <- *m
	
	lg.msg("Starting lvcl")
	//lg.delLogger()
	}

func mainLoop(){
	//time.Sleep(time.Duration(config.ClusterTickInterval))
	for i:=0;i<10;i++ {
		fmt.Println(i)
		time.Sleep(time.Second)}
		//time.Sleep(time.Duration(1000*1000*100*i))}
		
	}
