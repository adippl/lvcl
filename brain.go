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
	brainRpcElectReject
	brainRpcElectAccept
	brainRpcElectWasNominatedBy
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
	brainIN		<-chan message
	exchangeIN		chan message
	nodeHealth	map[string]int
	nodeHealthLast30Ticks	map[string][]int
	}




func NewBrain(exIN chan message, bIN <-chan message) *Brain {
	b := Brain{
		active: false,
		isMaster: false,
		masterNode:	nil,
		killBrain:	false,
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
		fmt.Printf("DEBUG BRAIN received message %+v\n", m)

		if m.RpcFunc == brainRpcAskForMasterNode{
			b.replyToAskForMasterNode(&m)
		}else if m.RpcFunc == brainRpcHaveMasterNodeReply {
			b.clusterHasMaster = true
			b.masterNode=&m.Argv[0]
		}else if m.RpcFunc == brainRpcHaveMasterNodeReplyNil {
			b.clusterHasMaster = false
			b.masterNode=nil
		}}}

func (b *Brain)replyToAskForMasterNode(m *message){
	newM := message{
		SrcHost: config.MyHostname,
		DestHost: m.SrcHost,
		SrcMod: msgModBrain,
		DestMod: msgModBrain,
		RpcFunc: brainRpcHaveMasterNodeReply,
		Time: time.Now(),
		Argv: make([]string,1),
		Argc: 1,}
	if b.isMaster && *b.masterNode == config.MyHostname {
		newM.RpcFunc=brainRpcHaveMasterNodeReply
		newM.Argv[0] = config.MyHostname
	}else if b.masterNode == nil {
		newM.RpcFunc=brainRpcHaveMasterNodeReplyNil
		newM.Argv[0]="cluster Doesn't currently have a masterNodea"}
	b.exchangeIN <- newM}

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
		b.PrintNodeHealth()
		fmt.Printf("highest weight, healthy node found %s\n", *b.findHighWeightNode())
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

func (b *Brain)askClusterForMasterNode(){
	var m = message{
		SrcHost: config.MyHostname,
		DestHost: "__everyone__",
		SrcMod: msgModBrain,
		DestMod: msgModBrain,
		RpcFunc: brainRpcAskForMasterNode,
		Time: time.Now(),
		Argc: 1,
		Argv: []string{"askForMasterNode"},
		}
	e.exchangeIN <- m}

func	(b *Brain)getMasterNode(){
	//wait for cluster to stabilize after start
	time.Sleep(time.Second*5)
	for{
		if b.killBrain {
			return}
		if b.masterNode == nil {
			fmt.Println("looking for master node")
			b.askClusterForMasterNode()
			time.Sleep(time.Millisecond * 500)
			continue}
		if b.masterNode == nil && b.clusterHasMaster == true {
			//TODO start cluster elections
			}
		time.Sleep(time.Millisecond * 1000) }
		}

