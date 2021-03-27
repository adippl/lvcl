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
	HealthGreen=iota
	HealthOrange
	HealthRed
	)


type Brain struct{
	active	bool
	master	bool
	killBrain	bool
	brainIN		<-chan message
	exIN		chan message
	nodeHealth	map[string]int
	heartbeat	map[string]*time.Time
	}




func NewBrain(exIN chan message, bIN <-chan message) *Brain {
	b := Brain{
		active: false,
		master: false,
		killBrain:	false,
		brainIN:	bIN,
		exIN:		exIN,}
	go b.updateNodeHealth()
	go b.messageHandler()
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
		}}

func (m *message)ValidateMessageBrain() bool {
	if	m.SrcMod != msgModBrain ||
		m.DestMod != msgModBrain ||
		m.DestHost != config.MyHostname {
		//config.checkIfNodeExists(&m.SrcHost) != false { //TODO consider adding check for SrcHost
			return false}
	return true}

//this function is very naive
//TODO add something better, for example node health Promotion only after X cycles of good behaviour
func (b *Brain)updateNodeHealth(){	//TODO, add node load to health calculation
	var dt time.Duration
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
			if dt> (time.Millisecond * time.Duration(config.ClusterTickInterval * 2)){
				b.nodeHealth[k]=HealthRed
				continue}
			if dt > (time.Millisecond * time.Duration(config.ClusterTickInterval)) {
				b.nodeHealth[k]=HealthOrange
				continue}
			b.nodeHealth[k]=HealthGreen
				
		}
		b.PrintNodeHealth()
		time.Sleep(time.Millisecond * time.Duration(config.NodeHealthCheckInterval))
		}}

func (b *Brain)PrintNodeHealth(){
	fmt.Printf("=== Node Health ===\n")
	for k,v := range b.nodeHealth {
		switch v {
			case HealthGreen:
				fmt.Printf("node: %s health: %s\n",k,"Green")
			case HealthOrange:
				fmt.Printf("node: %s health: %s\n",k,"Orange")
			case HealthRed:
				fmt.Printf("node: %s health: %s\n",k,"Red")
				}}
	fmt.Printf("===================\n")}

//func (m *message)findHighValueNode(){
//	}
