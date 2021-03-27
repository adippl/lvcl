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
import "math/rand"

const(
	HealthGreen=1
	HealthOrange=2
	HealthRed=5
	)
const(
	brainRpcElectNominate=iota
	brainRpcElectReject
	brainRpcElectAccept
	brainRpcElectWasNominatedBy
	brainRpcElectAsk
	brainRpcElectAskReplyYes
	brainRpcElectAskReplyNo
	brainRpcAskForMasterNode
	brainRpcHaveMasterNodeReply
	brainRpcHaveMasterNodeReplyNil
	)

type Brain struct{
	active	bool
	isMaster	bool
	clusterHasMaster	bool
	masterNode	*string
	
	killBrain	bool
	elections	bool
	nominated	bool
	nominatedBy	map[string]bool
	voteCounterExists	bool
	
	brainIN		<-chan message
	exchangeIN		chan message
	nodeHealth	map[string]int
	nodeHealthLast30Ticks	map[string][]int
	//TODO ADD ELECTION TIMEOUT
	}




func NewBrain(exIN chan message, bIN <-chan message) *Brain {
	b := Brain{
		active: false,
		isMaster: false,
		masterNode:	nil,
		killBrain:	false,
		elections:	false,
		nominated:	false,
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
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	for {
		if b.killBrain == true{
			return}
		m = <-b.brainIN
		//if config.DebugNetwork{
		//	fmt.Printf("DEBUG BRAIN received message %+v\n", m)}
		fmt.Printf("DEBUG BRAIN received message %+v\n", m)

		if m.RpcFunc == brainRpcAskForMasterNode{
			b.replyToAskForMasterNode(&m)
		}else if m.RpcFunc == brainRpcHaveMasterNodeReply {
			b.clusterHasMaster = true
			b.masterNode=&m.Argv[0]
		}else if m.RpcFunc == brainRpcHaveMasterNodeReplyNil {
			b.clusterHasMaster = false
			b.masterNode=nil
			//random length delay to make collisions less likely
			time.Sleep(time.Millisecond * time.Duration(100 * r1.Intn(10)))
			b.SendMsg("__everyone__",brainRpcElectAsk,"asking for elections")
		}else if m.RpcFunc == brainRpcElectAsk {
			b.replyToAskForElections(&m)
		}else if m.RpcFunc == brainRpcElectAskReplyYes {
			//TODO voter goroutine
			b.vote()
		}else if m.RpcFunc == brainRpcElectAskReplyNo {
			b.vote()
			//hostname := b.findHighWeightNode()
			////if !b.nominated{
			//if !b.nominated{
			//	nm := brainNewMessage()
			//	nm.DestHost = "__everyone__"
			//	nm.RpcFunc=brainRpcElectNominate
			//	nm.Argc=2
			//	nm.Argv=make([]string,2)
			//	nm.Argv[0]=*hostname
			//	nm.Argv[1]=fmt.Sprintf("nominating node %s",*hostname)
			//	b.exchangeIN <- *nm
			//	b.nominated=true}
		}else if m.RpcFunc == brainRpcElectNominate {
			//b.elections = true
			if config.MyHostname == m.Argv[0] {
				b.nominatedBy = make(map[string]bool)
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
		fmt.Println("VOTE ALREADY RUNNING")
		return}
	fmt.Println("VOTE COUNTER STARTED")
	b.voteCounterExists = true
	time.Sleep(time.Second * 5)
	var sum uint = 0
	for k,v := range b.nominatedBy {
		fmt.Println("nominated by",k,v)
		if v {
			sum++}}
	if sum == config.Quorum-1 {
		//lg.msg("this host won elections with quorum-1 of votes")
		fmt.Println("this host won elections with quorum-1 of votes")
		b.isMaster = true
		b.masterNode = &config.MyHostname
		b.elections = false
		b.nominated = false
		b.nominatedBy = make(map[string]bool)
	}else{
		fmt.Printf("elections failed, not enough votes (quorum-1), %+v",b.nominatedBy)
		lg.msg(fmt.Sprintf(
			"elections failed, not enough votes (quorum-1), %+v",
			b.nominatedBy))}
	fmt.Println("VOTE COUNTER EXITED")
	b.voteCounterExists = false}


func (b *Brain)replyToAskForMasterNode(m *message){
	newM := brainNewMessage()
	newM.DestHost = m.SrcHost
	newM.RpcFunc=brainRpcHaveMasterNodeReply
	if b.isMaster && *b.masterNode == config.MyHostname {
	newM.RpcFunc=brainRpcHaveMasterNodeReply
		newM.Argv[0] = config.MyHostname
	}else if b.masterNode == nil {
		newM.RpcFunc=brainRpcHaveMasterNodeReplyNil
		newM.Argv[0]="cluster Doesn't currently have a masterNode"}
	b.exchangeIN <- *newM}

func (b *Brain)replyToAskForElections(m *message){
	newM := brainNewMessage()
	newM.DestHost = m.SrcHost
	newM.RpcFunc=brainRpcHaveMasterNodeReply
	if b.elections {
		newM.RpcFunc=brainRpcElectAskReplyYes
		newM.Argv[0] = "cluster elections active"
	}else if !b.elections && b.masterNode != nil {
		newM.RpcFunc=brainRpcElectAskReplyNo
		newM.Argv[0] = "cluster doesn't have elections"}
	b.exchangeIN <- *newM}

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
		//fmt.Printf("highest weight, healthy node found %s\n", *b.findHighWeightNode())
		time.Sleep(time.Millisecond * time.Duration(config.NodeHealthCheckInterval))}}

func (b *Brain)PrintNodeHealth(){
	fmt.Printf("=== Node Health ===\n")
	for k,v := range b.nodeHealth {
		switch v {
			case HealthGreen:
				fmt.Printf("node: %s health: %s\n",k,"Green")
			case HealthOrange:
				fmt.Printf("node: %s health: %s\n",k,"Orange")
			case HealthRed:
				fmt.Printf("node: %s health: %s\n",k,"Red")}}
	fmt.Printf("===================\n")}

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

func	(b *Brain)getMasterNode(){
	//wait for cluster to stabilize after start
	//time.Sleep(time.Second*2)
	for{
		if b.killBrain {
			return}
		time.Sleep(time.Millisecond * 1000)
		if b.masterNode == nil && b.elections == false {
			fmt.Println("=================== asking for elections")
			fmt.Println("looking for master node")
			b.SendMsg("__everyone__", brainRpcAskForMasterNode, "asking for master node")
			time.Sleep(time.Millisecond * 500)}}}

func (b *Brain)SendMsg(host string, rpc uint, str string){
	var m *message = brainNewMessage()
	m.DestHost=host
	m.RpcFunc=rpc
	m.Argv = []string{str}
	e.exchangeIN <- *m}

