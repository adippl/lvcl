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

//import "time"
import "fmt"
import "strings"
import "os"

func main(){
	var args0split []string
	var progname string
	
	// get name of the program
	fmt.Println("arg0", os.Args[0])
	args0split = strings.Split(os.Args[0], "/")
	progname = args0split[len(args0split)-1]
	
	if progname == "lvcl" {
		fmt.Println("lvcl starting in daemon mode")
		daemonSetup()
		os.Exit(0)
	}else if progname == "lvcl-client" {
		//fmt.Println("lvcl started in client mode")
		client()
		os.Exit(0)
	}else{
		fmt.Fprintln(os.Stderr, "incorrect filename!")
		os.Exit(1)}}
