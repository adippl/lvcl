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
import "runtime"

var lg *Logger

type Logger struct{
	setupDone	bool
	logLocal	*os.File
	logCombined	*os.File
	killLogger	bool
	
	loggerIN	chan message
	exchangeIN	chan<- message
	}

func NewLoger(lIN chan message, exIN chan<- message) *Logger{
	var l Logger
	l.setupDone=true
	l.killLogger=false
	l.exchangeIN=exIN
	l.loggerIN=lIN
	
	f,err := os.OpenFile(config.LogLocal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)}
	l.logLocal=f

	f,err = os.OpenFile(config.LogCombined, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)}
	l.logCombined=f
	go l.messageHandler()
	fmt.Println("logger setup end")
	return &l}

func (l *Logger)KillLogger(){
	l.killLogger=true}

func (l *Logger)delLogger(){
	fmt.Println("CLOSING LOGGER")
	l.killLogger=true
	l.logLocal.Close()
	l.logCombined.Close()
	l=nil}
	
func (l *Logger)messageHandler(){
	var newS string
	var m message
	for {
		if l.killLogger == true{
			return}
		m = <-l.loggerIN
		if config.DebugNetwork {
			fmt.Printf("DEBUG LOGGER received message %+v\n", m)}
		if m.loggerMessageValidate(){
			newS = fmt.Sprintf("[src: %s][time: %s] %s \n", m.SrcHost, m.Time.String(), m.Argv[0])
			if config.DebugRawLogging {//debug option, separate in overwriting existing string
				newS = fmt.Sprintf("%+v\n", m)}
			_,err := l.logCombined.WriteString(newS)
			if err != nil {
				panic(err)}
		}else{
			l.msg("ERR message failed to validate: \"" + m.Argv[0] + "\"\n")}}}

func (m *message)loggerMessageValidate() bool { // TODO PLACEHOLDER
	return true}
	
func (l *Logger)msg(arg string){
	var s string
	var t = time.Now()
	if l.setupDone == false {
		fmt.Printf("WARNING Logging before log setup %s\n", arg)
		return}
	s = fmt.Sprintf("[src: %s][time: %s] %s \n", config.MyHostname, t, arg)
		

	fmt.Println(s)
	_,err := l.logLocal.WriteString(s)
	if err != nil{
		fmt.Println(err)
		panic(err)}
	if config.DebugNoRemoteLogging == false {
		msg := msgFormat(&arg)
		if config.DebugNetwork {
			fmt.Printf("DEBUG LOGGER Sending message %+v\n", *msg)}
		l.exchangeIN <- *msg}
	}

func (l *Logger)err(s string, e error){
	pc := make([]uintptr, 10)  // at least 1 entry needed
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])
	str := fmt.Sprintf("%s:%d %s\n", file, line, f.Name())
	l.msg(fmt.Sprintf("ERR %s - %s - %s",str , s, e))}

func msgFormat(s *string) *message{
	var m message
	
	m.SrcHost=config.MyHostname
	m.DestHost="__everyone__"
	m.SrcMod=msgModLoggr
	m.DestMod=msgModLoggr
	m.RpcFunc=1
	m.Time=time.Now()
	m.Argc=1
	m.Argv=append(m.Argv,*s)
	
	return &m}

func (l *Logger)DEBUGmessage(m *message){
	str := fmt.Sprintf("\nDEBUG logged message: %+v \n", *m)
	nmsg := msgFormat(&str)
	l.loggerIN <- *nmsg
	}

