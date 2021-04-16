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

//import "gitlab.com/libvirt/libvirt-go@v7.0.0"
import libvirt "gitlab.com/libvirt/libvirt-go"
import "fmt"

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
	}

var lv *lvd

func NewLVD(a_brainIN chan<- message, a_lvdIN <-chan message) *lvd {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		lg.err("libvirt NewConnect Local",err)
		defer conn.Close()
		return nil
		}
	// TODO close this connection at some point
	// defer close doesn't defer and closes connection immediately
	//defer conn.Close()
	
	l_lvd := lvd{
		brainIN: a_brainIN,
		lvdIN: a_lvdIN,
		daemonConneciton: conn,
		}
	return &l_lvd }

func (l *lvd)listDomains(){
	if l == nil {
		fmt.Println("lvd object ptr == nil")
		return}
	doms, err := l.daemonConneciton.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE)
	if err != nil {
	    lg.err("libvirt listAllDomains",err)
		return}
	
	fmt.Println(doms)
	fmt.Printf("%d running domains:\n", len(doms))
	for _, dom := range doms {
		name, err := dom.GetName()
		if err == nil {
			fmt.Printf("  %s\n", name) }
		dom.Free() }}



//https://gitlab.com/libvirt/libvirt-go.git
