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

const(
	HealthGreen=1
	HealthOrange=2
	HealthRed=5
	)
const(
	brainRpcElectNominate=iota
	brainRpcElectAsk
	brainRpcAskForMasterNode
	brainRpcHaveMasterNodeReply
	brainRpcHaveMasterNodeReplyNil
	)

type Brain struct{
	isMaster	bool
	masterNode	*string
	
	killBrain	bool
	nominatedBy	map[string]bool
	voteCounterExists	bool
	
	brainIN		<-chan message
	exchangeIN		chan message
	nodeHealth	map[string]int
	nodeHealthLast30Ticks	map[string][]int
	}


var b *Brain

func NewBrain(exIN chan message, bIN <-chan message) *Brain {
	b := Brain{
		isMaster: false,
		masterNode:	nil,
		killBrain:	false,
		nominatedBy:	make(map[string]bool),
		voteCounterExists: false,
		
		brainIN:	bIN,
		exchangeIN:		exIN,
		nodeHealth: make(map[string]int),
		nodeHealthLast30Ticks:	make(map[string][]int),
		}
	go b.updateNodeHealth()
	go b.messageHandler()
	go b.getMasterNode()
	return &b}

func (b *Brain)KillBrain(){
	b.killBrain=true}

func  (b *Brain)messageHandler(){
	var m message
	for {
		if b.killBrain == true{
			return}
		m = <-b.brainIN
		//if config.DebugNetwork{
		//	fmt.Printf("DEBUG BRAIN received message %+v\n", m)}
		//lg.msg(fmt.Sprintf("DEBUG BRAIN received message %+v\n", m))
		
		if m.RpcFunc == brainRpcAskForMasterNode{
			b.replyToAskForMasterNode(&m)
			continue
		
		// Master node only replies
		}else if b.isMaster &&
				(m.RpcFunc == brainRpcElectAsk ||
				 m.RpcFunc == brainRpcElectNominate) {
				b.SendMsg(m.SrcHost, brainRpcHaveMasterNodeReply, config.MyHostname)
				continue
		// respond to message with info about master node
		}else if m.RpcFunc == brainRpcHaveMasterNodeReply {
			// make a copy of this string
			// I had some strange (garbage collection?) problems with out this copy
			str := m.Argv[0]
			b.masterNode=&str
			lg.msg(fmt.Sprintf("got master node %s from host %s",m.Argv[0],m.SrcHost))
		// respond to brainRpcHaveMasterNodeReplyNil
		}else if m.RpcFunc == brainRpcHaveMasterNodeReplyNil {
			b.masterNode = nil
			b.SendMsg("__everyone__",brainRpcElectAsk,"asking for elections")
			continue
		// respond to request for elections
		}else if m.RpcFunc == brainRpcElectAsk {
			b.vote()
			continue
		// respond to master node nomination
		}else if m.RpcFunc == brainRpcElectNominate {
			if config.MyHostname == m.Argv[0] {
				//this node got nominated
				b.nominatedBy[m.SrcHost]=true
				go b.countVotes()
			}}}}

func (b *Brain)vote(){
	hostname := b.findHighWeightNode()
	nm := brainNewMessage()
	nm.DestHost = "__everyone__"
	nm.RpcFunc=brainRpcElectNominate
	nm.Argc=2
	nm.Argv=make([]string,2)
	nm.Argv[0]=*hostname
	nm.Argv[1]=fmt.Sprintf("nominating node %s",*hostname)
	b.exchangeIN <- *nm
	}

func (b *Brain)countVotes(){
	if b.voteCounterExists {
		lg.msg("recieved ask for vote, but vote coroutine is already running")
		return}
	b.voteCounterExists = true
	time.Sleep(time.Millisecond * 1000)
	var sum uint = 0
	for k,v := range b.nominatedBy {
		lg.msg(fmt.Sprintf("this node (%s) nominated by $s $B ",k,v))
		if v {
			sum++}}
	if sum == config.Quorum-1 {
		fmt.Println("this host won elections with (quorum - 1) of votes")
		b.isMaster = true
		b.masterNode = &config.MyHostname
		b.nominatedBy = make(map[string]bool)
		//fmt.Println("DEBUG WON ", b.isMaster, *b.masterNode, b.nominatedBy)
	}else{
		lg.msg(fmt.Sprintf(
			"elections failed, not enough votes (quorum-1), %+v",
			b.nominatedBy))}
	b.nominatedBy = make(map[string]bool)
	b.voteCounterExists = false}

func (b *Brain)replyToAskForMasterNode(m *message){
	if b.isMaster && *b.masterNode == config.MyHostname {
		//fmt.Println("\n\nreplying with master node\n\n")
		b.SendMsg(m.SrcHost, brainRpcHaveMasterNodeReply, config.MyHostname)
	}else if b.masterNode == nil {
		b.SendMsg(m.SrcHost,
			brainRpcHaveMasterNodeReplyNil,
			"cluster Doesn't currently have a masterNode")}}

//func (b *Brain)replyFromMasterNode(m *message){
//	if b.isMaster && *b.masterNode == config.MyHostname {
//		b.SendMsg(m.SrcHost, brainRpcHaveMasterNodeReply, config.MyHostname)}}

func (m *message)ValidateMessageBrain() bool {
	if	m.SrcMod != msgModBrain ||
		m.DestMod != msgModBrain ||
		( m.DestHost != config.MyHostname && m.DestHost != "__everyone__") {
		//config.checkIfNodeExists(&m.SrcHost) != false { //TODO consider adding check for SrcHost
			return false}
	return true}

//this function is very naive
//node health is calculated form average of last 30 check intervals
//this code is not very optimized, but it's good enough
func (b *Brain)updateNodeHealth(){	//TODO, add node load to health calculation
	var dt time.Duration
	var sum int
	var avg float32
	for{
		if b.killBrain {
			return}
		//make new map, it's safer than removing dropped nodes from old map
		b.nodeHealth = make(map[string]int)
		b.nodeHealth[config.MyHostname]=HealthGreen
		for k,v := range e.GetHeartbeat() {
			//get absolute value of time.
			//in this simple implemetation time can be negative due to time differences on host
			dt = *v
			if dt < 0 {
				dt = 0 - dt}
			if dt> (time.Millisecond * time.Duration(config.NodeHealthCheckInterval * 2)){
				b.nodeHealthLast30Ticks[k]=append(b.nodeHealthLast30Ticks[k], HealthRed)
			}else if dt > (time.Millisecond * time.Duration(config.NodeHealthCheckInterval)) {
				b.nodeHealthLast30Ticks[k]=append(b.nodeHealthLast30Ticks[k], HealthOrange)
			}else {
				b.nodeHealthLast30Ticks[k]=append(b.nodeHealthLast30Ticks[k], HealthGreen)
				}
			sum=0
			for _,x := range b.nodeHealthLast30Ticks[k] {
				sum = sum + x}
			avg = float32(sum) / float32(len(b.nodeHealthLast30Ticks[k]))
			if avg>2 && avg<=3 {
				b.nodeHealth[k]=HealthRed
			}else if avg>1 && avg<=2 {
				b.nodeHealth[k]=HealthOrange
			}else if avg >= 1 && avg <= 1.1 {
				b.nodeHealth[k]=HealthGreen}
			//remove last position if slice size gets over 29
			if len(b.nodeHealthLast30Ticks[k]) > 29 {
				b.nodeHealthLast30Ticks[k] = b.nodeHealthLast30Ticks[k][1:]}}
		//b.PrintNodeHealth()
		time.Sleep(time.Millisecond * time.Duration(config.NodeHealthCheckInterval))}}

func (b *Brain)findHighWeightNode() *string {
	var host *string
	var hw uint = 0
	for k,v := range b.nodeHealth{
		n := config.getNodebyHostname(&k)
		if v == HealthGreen && n != nil && n.Weight > hw {
			hw=n.Weight
			host=&n.Hostname}}
	return host}

func brainNewMessage() *message {
	var m = message{
		SrcHost: config.MyHostname,
		SrcMod: msgModBrain,
		DestMod: msgModBrain,
		Time: time.Now(),
		Argc: 1,
		Argv: make([]string,1)}
	return &m}

func (b *Brain)getMasterNode(){
	for{
		if b.killBrain {
			return}
		time.Sleep(time.Millisecond * 1000)
		if b.masterNode == nil {
			lg.msg("looking for master node")
			b.SendMsg("__everyone__", brainRpcAskForMasterNode, "asking for master node")
			time.Sleep(time.Millisecond * 500)}}}

func (b *Brain)SendMsg(host string, rpc uint, str string){
	var m *message = brainNewMessage()
	m.DestHost=host
	m.RpcFunc=rpc
	m.Argv = []string{str}
	e.exchangeIN <- *m}


func (b *Brain)PrintNodeHealth(){
	fmt.Printf("=== Node Health ===\n")
	if b.masterNode != nil {
		fmt.Printf("Master node: %s\n", *b.masterNode)
	}else{
		fmt.Printf("No master node\n")}
	for k,v := range b.nodeHealth {
		switch v {
			case HealthGreen:
				fmt.Printf("node: %s health: %s\n",k,"Green")
			case HealthOrange:
				fmt.Printf("node: %s health: %s\n",k,"Orange")
			case HealthRed:
				fmt.Printf("node: %s health: %s\n",k,"Red")}}
	fmt.Printf("===================\n")}

//func (b *Brain)PrintNodeHealth(){
//	//fmt.Printf("=== Node Health ===\n")
//	if b.masterNode != nil {
//		lg.msg(fmt.Sprintf("Master node: %s\n", *b.masterNode))
//	}else{
//		lg.msg("No master node")}
//	//for k,v := range b.nodeHealth {
//	//	switch v {
//	//		case HealthGreen:
//	//			fmt.Printf("node: %s health: %s\n",k,"Green")
//	//		case HealthOrange:
//	//			fmt.Printf("node: %s health: %s\n",k,"Orange")
//	//		case HealthRed:
//	//			fmt.Printf("node: %s health: %s\n",k,"Red")}}
//	//fmt.Printf("===================\n")
//	}
//
