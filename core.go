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
	if config.getNodebyHostname(&config.MyHostname) == nil {
		fmt.Printf("CURRENT HOST [%s] IS NOT IN CONFIG, EXITTING", config.MyHostname)
		panic("WRONG HOSTNAME")}
	brainIN:=make(chan message)
	loggerIN:=make(chan message)
	exchangeIN:=make(chan message)
	
	lg = NewLoger(loggerIN, exchangeIN)
	
	e = NewExchange(exchangeIN, brainIN, loggerIN)
	e.placeholderStupidVariableNotUsedError()
	
	b = NewBrain(exchangeIN, brainIN)
	
	lg.msg("Starting lvcl")
	
	mainLoop()
	e.printHeartbeatStats()
	e.dumpAllConnectedHosts()
	b.KillBrain()
	lg.KillLogger()
	e.KillExchange()
	//lg.delLogger()
	fmt.Println("program should've closed all files and connections by now")
	time.Sleep(time.Second*10)
	}

func mainLoop(){
	for i:=0;i<15;i++ {
		fmt.Println(i)
		lg.msg(fmt.Sprintf("%d",i))
		b.PrintNodeHealth()
		time.Sleep(time.Second)}
	}
