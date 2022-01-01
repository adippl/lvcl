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
//import "json"

const(
	msgModCore=iota
	msgModLoggr
	msgModExchn
	msgModBrain
	msgModExchnHeartbeat
	msgModBrainController
	msgModClient
	msgModConfig
	
	rpcHeartbeat
	brainRpcElectNominate
	brainRpcElectAsk
	brainRpcAskForMasterNode
	brainRpcHaveMasterNodeReply
	brainRpcHaveMasterNodeReplyNil
	brainRpcSendingStats
	brianRpcSendingClusterResources
	brianMasterSendUpdatedResources
	exchangeAskAboutClientNode
	exchangeNotifyAboutClient
	exchangeNotifyClientDisconnect
	exchangeSendClientID
	clientAskAboutStatus
	clientAskAboutStatusReply
	clientListenToClusterLogger
	loggerForwardMessageToClient
	loggerForwardMessageToClientStop
	clientPrintText
	clientPrintTextLogger
	clientPrintTextStatus
	clientAskResStateChange
	clientAskResStateChangeReply
	confUpdateResState
	confNotifAboutEpoch
	confNotifAboutEpochUpdate
	confNotifAboutEpochUpdateAsk
	brainNotifyAboutEpoch
	brainNotifyAboutEpochUpdate
	brainNotifyAboutEpochUpdateAsk
	brainNotifyMasterAboutResourceFailure
	brainNotifyMasterAboutLocalResources
	)

type message struct{
	SrcHost		string
	DestHost	string
	SrcMod		uint
	DestMod		uint
	ConfHash	string
	Time		time.Time
	RpcFunc		uint
	Argv		[]string
	Custom1		interface{}
	Custom2		interface{}
	Custom3		interface{}
	}



func (pm *message)dump(){
	fmt.Printf("%+v \n", *pm)}

/* TODO replace with something faster */
/* TODO ADD LOGGER LOGGING */
//func (pm *message)validate() bool {
	
func Newmessage() *message {
	var m message
	m.SrcHost=config.MyHostname
	return &m}

func (m *message)setStr(s *string){
	m.Argv=append(m.Argv,*s)}

func (m *message)heartbeatGetTime() *time.Time {
	t,err := time.Parse(config.HeartbeatTimeFormat, m.Argv[0])
	if err != nil {
		lg.err("heartbeatGetTime time.Parse error", err)}
	return &t}
