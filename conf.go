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
import "os"
import "encoding/json"
import "io/ioutil"
import "errors"
import "crypto/sha256"
import "io"

const( confDir="./" )
const( confFile="cluster.json" )

type Conf struct {
	UUID string
	DomainDefinitionDir string
	Nodes []Node
	VMs []VM
	Quorum uint `json:"test"`
	BalanceMode uint
	ResStickiness uint
	GlobMigrationTimeout uint
	GlobLiveMigrationBlock bool
	Maintenance bool

	VCpuMax uint
	HwCpuMax uint
	VMemMax uint
	HwMemMax uint
	
	HeartbeatInterval uint
	ClusterTickInterval uint
	TCPport string
	UnixSocket string
	ConfFileHash string `json:"-"`
	ConfFileHashRaw []byte `json:"-"`
	MyHostname string `json:"-"`

	LogLocal string
	LogCombined string
	}


//func NewConf() Conf {
//   conf := Something{}
//   return conf
//func NewConf()*Conf{
//	return &Conf{}

var config Conf

func confPrint(p *Conf){
	fmt.Printf("%+v \n", *p)
	}

func confLoad(){
	file, err := os.Open("cluster.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(10)}
	defer file.Close()
	fmt.Println("reading cluster.json")	
	raw, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("ERR reading cluster.json")
		os.Exit(10);}
		
	config.MyHostname,err=os.Hostname()
	if(err != nil){
		panic(err)}
	
	/* TODO DEBUG 
	 * this codes tests if current hosts is in config file. 
	 * exits program if host is not in config
	_,err=config.getNodebyHostname(&config.MyHostname)
	if(err == nil){
		fmt.Println("DEBUG err controller is started on hostname %s which doesn't exists in cluster configuration file\n",vm.DomainDefinition)
		os.Exit()}
	 */
	
	var h=sha256.New()
	if _,err:=io.Copy(h,file);err!=nil {
		fmt.Println("ERR hashing cluster.json")}
	config.ConfFileHash=fmt.Sprintf("%x",h.Sum(nil))
	config.ConfFileHashRaw=h.Sum(nil)
		
	
	json.Unmarshal(raw,&config)
	
	loadAllVMfiles()
	
	fmt.Println("\n\n\n\n LOADED CONFIG\n\n\n\n")
	dumpConfig()

	}

func loadAllVMfiles(){
	f,e:=ioutil.ReadDir("domains")
	if e!=nil{
		fmt.Println(e)}
	for _, f := range f{
		VMReadFile("domains/"+f.Name())}}

func dumpConfig(){
	raw, _ := json.MarshalIndent(&config,"","	")
	fmt.Println(string(raw))
	}

func (c *Conf)getVMbyName(argName *string)(v *VM, err error){
	for _,t:= range c.VMs{
		if t.Name == *argName{
			return &t, nil}}
	return nil,errors.New("conf VM not found")}

func (c *Conf)getVMbyDomain(argDomain *string)(v *VM, err error){
	for _,t:= range c.VMs{
		if t.DomainDefinition == *argDomain{
			return &t, nil}}
	return nil,errors.New("conf VM not found")}


func (c *Conf)getNodebyHostname(argHostname *string)(v *Node, err error){
	for _,t:= range c.Nodes{
		if t.Hostname == *argHostname{
			return &t, nil}}
	return nil,errors.New("conf Node not found")}
