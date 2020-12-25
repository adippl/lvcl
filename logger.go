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

import "os"
import "fmt"
import "time"

var lg *Logger

type Logger struct{
	setupDone bool
	logLocal *os.File
	logCombined *os.File
	
	loggerIN	chan<- message
	loggerOUT	<-chan message
	}

func newLoger(in chan<- message, out <-chan message) *Logger{
	var l Logger
	l.setupDone=true
	l.loggerIN=in
	l.loggerOUT=out
	
	f,err := os.OpenFile(config.LogLocal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)}
	l.logLocal=f

	f,err = os.OpenFile(config.LogCombined, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)}
	l.logCombined=f
	fmt.Println("logger setup end")
	return &l}


func (l *Logger)delLogger(){
	l.logLocal.Close()
	l.logCombined.Close()
	l=nil}
	
func (l *Logger)handlesMessages(){
	for m := range l.loggerOUT{
		if m.loggerMessageValidate(){
			_,err := l.logLocal.WriteString(m.Argv[0])
			if err != nil {
				panic(err)}
		}else{
			l.msg("message: \"" + m.Argv[0] + "\"\n")}}}
		

func (m *message)loggerMessageValidate() bool { // TODO PLACEHOLDER
	return true}
	
func (l Logger)msg(s string){
	if l.setupDone == false {
		fmt.Printf("WARNING Logging before log setup %s\n", s)
		return}
	time := time.Now() //TODO MOVE IT SOMEWHERE
	newS := fmt.Sprintf("[src: %s][time: %s] %s \n", config.MyHostname, time.String(), s)
	fmt.Println(newS)


	_,err := l.logLocal.WriteString(newS)
	if err != nil{
		fmt.Println(err)
		panic(err)}
	
	l.loggerIN <- msgFormat(&newS)
	}

func msgFormat(s *string) message{
	var m message
	
	m.SrcMod=msgModLoggr
	m.DestMod=msgModLoggr
	m.RpcFunc=1
	m.Argc=1
	m.Argv=append(m.Argv,*s)
	
	return m
	}
