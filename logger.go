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
	
	loggerIN	chan message
	exchangeIN	chan<- message
	}

func NewLoger(lIN chan message, exIN chan<- message) *Logger{
	var l Logger
	l.setupDone=true
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


func (l *Logger)delLogger(){
	fmt.Println("CLOSEING LOGGER")
	l.logLocal.Close()
	l.logCombined.Close()
	l=nil}
	
func (l *Logger)messageHandler(){
	var newS string
	for {
		for m := range l.loggerIN{
			if m.loggerMessageValidate(){
				newS = fmt.Sprintf("[src: %s][time: %s] %s \n", config.MyHostname, m.Time.String(), m.Argv[0])
				_,err := l.logLocal.WriteString(newS)
				if err != nil {
					panic(err)}
			}else{
				l.msg("ERR message failed to validate: \"" + m.Argv[0] + "\"\n")}}}}

func (m *message)loggerMessageValidate() bool { // TODO PLACEHOLDER
	return true}
	
func (l *Logger)msg(s string){
	if l.setupDone == false {
		fmt.Printf("WARNING Logging before log setup %s\n", s)
		return}
	newS := fmt.Sprintf("[src: %s] %s", config.MyHostname, s)
	fileString := fmt.Sprintf("[src: %s] %s\n", config.MyHostname, s)

	_,err := l.logLocal.WriteString(fileString)
	if err != nil{
		fmt.Println(err)
		panic(err)}
	fmt.Println(newS)
	msg := msgFormat(&newS)
	l.exchangeIN <- *msg
	}

func (l *Logger)msgE(s string, e error){
	l.msg(fmt.Sprintf("ERR %s - %s", s, e))}

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
	
	str := fmt.Sprintf("DEBUG logged message: %+v \n", *m)
	nmsg := msgFormat(&str)
	l.loggerIN <- *nmsg
	}

