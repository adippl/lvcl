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

//import libvirt "gitlab.com/libvirt/libvirt-go@v7.0.0"
//https://gitlab.com/libvirt/libvirt-go.git
import libvirt "gitlab.com/libvirt/libvirt-go"
import "fmt"
import "io/ioutil"

const(
	lvdVmStateNil=iota
	lvdVmStateStarting=iota
	lvdVmStateStarted
	lvdVmStatePaused
	lvdVmStateStopping
	lvdVmStateFsHalt
	lvdVmStateFsResume
	lvdVmState
	)

type lvd struct {
	brainIN					chan<- message
	lvdIN					<-chan message
	domainState				map[string]uint
	domainDesiredState		map[string]uint
	daemonConneciton		*libvirt.Connect
	nodeCPUStats				*libvirt.NodeCPUStats
	nodeMemStats				*libvirt.NodeMemoryStats
	}

type lvdVM struct {
	name	string
	state	uint
	}

var lv *lvd

func NewLVD(a_brainIN chan<- message, a_lvdIN <-chan message) *lvd {
	var err error
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		lg.err("libvirt NewConnect Local",err)
		defer conn.Close()
		return nil
		}
	// TODO close this connection at some point
	// defer close doesn't defer and closes connection immediately
	//defer conn.Close()
	
//	err = nil
//	cpustats, err := conn.GetCPUStats(int(libvirt.NODE_CPU_STATS_ALL_CPUS), 0)
//	if err != nil {
//		lg.err("libvirt GetCPUStats",err)
//		cpustats = nil}
//	
//	err = nil
//	memstats, err := conn.GetMemoryStats(int(libvirt.NODE_MEMORY_STATS_ALL_CELLS), 0)
//	if err != nil {
//		lg.err("libvirt GetMemoryStats",err)
//		memstats = nil}
	
	l_lvd := lvd{
		brainIN: a_brainIN,
		lvdIN: a_lvdIN,
		daemonConneciton: conn,
//		nodeCPUStats: cpustats,
//		nodeMemStats: memstats,
		}
	l_lvd.updateStats()
	return &l_lvd }

func (l *lvd)updateStats(){
	var err error
	err = nil
	cpustats, err := l.daemonConneciton.GetCPUStats(int(libvirt.NODE_CPU_STATS_ALL_CPUS), 0)
	if err != nil {
		lg.err("libvirt GetCPUStats",err)
		l.nodeCPUStats = nil
	}else{
		l.nodeCPUStats = cpustats }
	
	err = nil
	memstats, err := l.daemonConneciton.GetMemoryStats(int(libvirt.NODE_MEMORY_STATS_ALL_CELLS), 0)
	if err != nil {
		lg.err("libvirt GetMemoryStats",err)
		l.nodeMemStats = nil
	}else{
		l.nodeMemStats = memstats }}

func (l *lvd)listDomains(){
	if l == nil {
		fmt.Println("lvd object ptr == nil")
		return }
	l.updateStats()
	doms, err := l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
	    lg.err("libvirt listAllDomains",err)
		return}
	fmt.Printf("NodeCPUStats %+v\n",l.nodeCPUStats)
	fmt.Printf("NodeMemoryStats %+v\n",l.nodeMemStats)
	
	fmt.Printf("%d running domains:\n", len(doms))
	for _, dom := range doms {
		name, err := dom.GetName()
		if err == nil {
			fmt.Printf("  %s\n", name) }
		dom.Free() }}

func (l *lvd)startVM(v *VM) int {
	file, err := ioutil.ReadFile(v.DomainDefinition)
	if err != nil {
		lg.err("startVM", err)
		return 1}
	xml := string(file)

	// start modes TODO later
	// libvirt.DOMAIN_NONE
	// libvirt.DOMAIN_START_VALIDATE
	err = nil
	//dom,err := l.daemonConneciton.DomainCreateXML(xml, libvirt.DOMAIN_NONE)
	_,err = l.daemonConneciton.DomainCreateXML(xml, libvirt.DOMAIN_NONE)
	if err != nil {
		lg.err("startVM", err)
		return 1}
	return 0}
	

