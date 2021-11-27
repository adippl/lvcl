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
	setupDone		bool
	logLocal		*os.File
	logCombined		*os.File
	killLogger		bool
	
	ex_logIN		chan message
	localLoggerIN	chan message
	log_ex			chan<- message
	forwardToCli	bool
	}

func NewLoger(lIN chan message, exIN chan<- message) *Logger{
	var l Logger
	l.setupDone=true
	l.killLogger=false
	l.log_ex=exIN
	l.ex_logIN=lIN
	
	l.localLoggerIN=make(chan message)
	
	f,err := os.OpenFile(config.LogLocal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)}
	l.logLocal=f
	
	f,err = os.OpenFile(config.LogCombined, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)}
	l.logCombined=f
	go l.messageHandler()
	fmt.Println("ex_log setup end")
	return &l}

func (l *Logger)KillLogger(){
	var ex_logOp, loc_logOp bool
//	var ex_logK, loc_logK bool = false, false
	if l.killLogger {return}
	l.killLogger=true
	time.Sleep(time.Millisecond * time.Duration(100))
	if config.DebugLevel>2 {
		fmt.Println("Debug, KillLogger()")}
	l.logLocal.Close()
	l.logCombined.Close()
	//close(l.localLoggerIN)
	fmt.Println(l.log_ex)
	close(l.log_ex)
	//close(l.localLoggerIN)
	for{
		select{
			case _,ex_logOp = <-l.ex_logIN:
			case _,loc_logOp = <-l.localLoggerIN:
			}
		//break if all channels are closed
		if ! ( ex_logOp || loc_logOp ) {
			break
		}}}

func (l *Logger)message_forward_to_client(m *message){
	var new_m message
	return
	if l.forwardToCli {
		new_m = *m
		new_m.SrcMod = msgModLoggr
		new_m.DestMod = msgModClient
		l.log_ex <- new_m}}

func (l *Logger)messageHandler(){
	var newS string
	var m message
	var exOk, loc_logOk bool
	var err error
	exOk = true
	loc_logOk = true
	for {
		m=message{}
		if l.killLogger {
			return}
		select{
			//incoming messagess from other hosts
			case m,exOk = <-l.ex_logIN:
				//skip if received closing message
				if ! exOk {
					break}
				if config.DebugNetwork {
					fmt.Printf("DEBUG LOGGER received message %+v\n", m)}
				if m.logger_message_validate(){
					newS = fmt.Sprintf("[src: %s][time: %s] %s \n",
						m.SrcHost, m.Time.String(), m.Argv[0])
					//debug option, separate in overwriting existing string
					if config.DebugRawLogging {
						newS = fmt.Sprintf("%+v\n", m)}
					_,err := l.logCombined.WriteString(newS)
					if err != nil {
						panic(err)}
					fmt.Println(fmt.Sprintf("remote_print %s", newS))
					//if m.SrcHost != config._MyHostname(){
					//	l.message_forward_to_client(&m)}
					l.message_forward_to_client(&m)
				}else{
					l.msg("ERR message failed to validate: \"" + m.Argv[0] + "\"\n")}
			//incoming messages from local
			case m,loc_logOk = <-l.localLoggerIN:
				//skip if received closing message
				if ! loc_logOk {
					break}
				s := fmt.Sprintf("[src: %s][time: %s] %s \n",
					config.MyHostname, m.Time, m.Argv[0])
				fmt.Println("local_print",s)
				//write to local log
				_,err = l.logLocal.WriteString(s)
				if err != nil{
					//don't pannic if file is closed because of killed logger
					if lg.killLogger {
						fmt.Println(err)
					}else{
						panic(err)}}
				//write to combined log
				_,err = l.logCombined.WriteString(s)
				if err != nil{
					//don't pannic if file is closed because of killed logger
					if lg.killLogger {
						fmt.Println(err)
					}else{
						panic(err)}}
				l.message_forward_to_client(&m)}
		if(config.DebugLogger){
			fmt.Printf("\n\nD_E_B_U_G ex_log exOk =%b\n\n", exOk)
			fmt.Printf("\n\nD_E_B_U_G ex_log loc_logOk =%b\n\n", loc_logOk)}
		if !( exOk || loc_logOk ) {
			lg.msg("both of the logger channels are closed, deleting logger")
			l.KillLogger()
			return}
			}}

func (m *message)logger_message_validate() bool { // TODO PLACEHOLDER
	return (
		m.SrcHost != config._MyHostname() &&
		m.SrcMod == msgModLoggr &&
		m.DestMod == msgModLoggr &&
		m.RpcFunc == 1 &&
		m.Argc == 1 )}
 
func (l *Logger)msg(arg string){
	if l.setupDone == false {
		fmt.Printf("WARNING Logging before log setup %s\n", arg)
		return}
	if l.killLogger {
		fmt.Printf("Warning logging with killed logger | %s\n", arg)
		return}
	lmsg := msgFormat(&arg)
	lmsg.DestHost=config._MyHostname()
	l.localLoggerIN <- *lmsg
	if config.DebugNoRemoteLogging == false {
		msg := msgFormat(&arg)
		if config.DebugNetwork {
			fmt.Printf("DEBUG LOGGER Sending message %+v\n", *msg)}
		l.log_ex <- *msg}
	}

func (l *Logger)msgERR(s string){
	l.msg(fmt.Sprintf("ERR %s - %s", s))}

func (l *Logger)msg_debug(level uint, s string){
	if level <= config.DebugLevel {
		l.msg(fmt.Sprintf("debug(level:%d) %s ", level, s))}}

func (l *Logger)err(s string, e error){
	pc := make([]uintptr, 10)  // at least 1 entry needed
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])
	str := fmt.Sprintf("%s:%d %s\n", file, line, f.Name())
	l.msg(fmt.Sprintf("ERR %s - %s - %s",str , s, e))}

func msgFormat(s *string) *message{
	var m message
	
	m.SrcHost=config._MyHostname()
	m.DestHost="__everyone__"
	m.SrcMod=msgModLoggr
	m.DestMod=msgModLoggr
	m.RpcFunc=1
	m.Time=time.Now()
	m.Argc=1
	//m.Argv=append(m.Argv,*s)
	m.Argv=[]string{*s}
	
	return &m}

