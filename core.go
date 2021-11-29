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

import "fmt"
import "time"
import "os"

func daemonSetup(){
	writeExampleConfig()
	confLoad()
	if config.GetNodebyHostname(&config.MyHostname) == nil &&
		config.GetField_bool("debugRunOnAnyHost") { //debug option
		fmt.Fprintf(os.Stderr, "CURRENT HOST [%s] IS NOT IN CONFIG, EXITTING", 
			config.MyHostname)
		panic("WRONG HOSTNAME")}
	
	brain_exchange:=make(chan message)
	exchange_brain:=make(chan message)
	
	logger_exchange:=make(chan message)
	exchange_logger:=make(chan message)
	
	
	lg = NewLoger(exchange_logger, logger_exchange)
	e = NewExchange(brain_exchange, exchange_brain, logger_exchange, exchange_logger)
	e.placeholderStupidVariableNotUsedError()
	b = NewBrain(exchange_brain, brain_exchange)
	

	lg.msg("Starting lvcl")
	mainLoop()
	//e.printHeartbeatStats()
	//e.dumpAllConnectedHosts()
	os.Exit(0)
	b.KillBrain()
	lg.KillLogger()
	e.KillExchange()
	fmt.Println("program should've closed all files and connections by now")
	os.Exit(0)
	}

func mainLoop(){
	for i:=0; i<30; i++ {
		fmt.Println(i)
		lg.msg(fmt.Sprintf("%d",i))
		e.printHeartbeatStats()
		b.PrintNodeHealth()
		time.Sleep(time.Second)}
	}

func test_conf_rw(i int){
	var b bool
	fmt.Println("coooore", config.GetField_uint("VCpuMax"))
	config.SetField_uint("VCpuMax", uint(i))
	fmt.Println("coooore", config.GetField_uint("VCpuMax"))

	fmt.Println("coooore", config.GetField_string("UUID"))
	config.SetField_string("UUID", fmt.Sprintf("%d", i+10))
	fmt.Println("coooore", config.GetField_string("UUID"))

	if(i%2==0){
		b=true
	}else{
		b=false}
	fmt.Println("coooore", config.GetField_bool("DebugHeartbeat"))
	config.SetField_bool("DebugHeartbeat", b)
	fmt.Println("coooore", config.GetField_bool("DebugHeartbeat"))
	}
