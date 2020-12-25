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
//import "json"

const(
	msgModCore=iota
	msgModLoggr
	msgModExchn
	msgModBrain
	//mgsModUndef
	)

type message struct{
	SrcHost		string
	DestHost	string
	SrcMod		uint
	DestMod		uint
	RpcFunc		uint
	Argc		uint
	Argv		[]string
	}



func (pm *message)dump(){
	fmt.Printf("%+v \n", *pm)
	}

	/* TODO replace with something faster */
	/* TODO ADD LOGGER LOGGING */
func (pm message)validate() bool {
	var err error
	_,err = config.getNodebyHostname(& pm.SrcHost)
	if err != nil {
		return true}
	
	_,err = config.getNodebyHostname(& pm.DestHost)
	if err != nil {
		return true}

	if ((pm.SrcMod != msgModCore)   &&
		(pm.SrcMod != msgModLoggr)  &&
		(pm.SrcMod != msgModExchn)  &&
		(pm.SrcMod != msgModBrain)) ||
	   ((pm.DestMod != msgModCore)  &&
		(pm.DestMod != msgModLoggr) &&
		(pm.DestMod != msgModExchn) &&
		(pm.DestMod != msgModBrain)) {
		return true}
	/* TODO module specific validation functions */
	return false}
	
	
	
	
	
	
	

