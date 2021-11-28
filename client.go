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
import "flag"
import "os"
import "runtime"
import "net"
//import "math/rand"
import "time"
import "crypto/sha256"
import "io"
import "strconv"


func clogger(l int, s string){
	if l==0 {
		fmt.Println(s)
	}else if l<=cliLogLevel {
		fmt.Fprintln(os.Stderr, s)}}
		
func cerr(s string, e error){
	pc := make([]uintptr, 10)  // at least 1 entry needed
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])
	lines := fmt.Sprintf("%s:%d %s\n", file, line, f.Name())
	fmt.Fprintln(os.Stderr, lines, s, e, "\n")}

var cliLogLevel			int
var ec					eclient
var confFileHash		string
var confFileHashRaw		[]byte
var myHostname			string
var clientID			string
var clientIDSet			bool = false
//	var cincoming		<-chan message = make(chan message)
//	var coutgoing		chan<- message = make(chan message)
var cincoming		chan message
var coutgoing		chan message
var exitAfterTextReply	bool = false

func waitForClientID(){
	for i:=0; i<100; i++ {
		if ! clientIDSet {
			time.Sleep(time.Duration(5) * time.Millisecond)
		}else{
			return}
		}
	clogger(1, "client didn't receive ID from cluster")
	os.Exit(3)}

func clientMessageHandler(){
	var m message
	for {
		m = <-cincoming
		if m.msg_handle_cluste_assigned_id() {continue}
		if m.msg_handle_client_print() {continue}
		fmt.Println(fmt.Sprintf("DEBUG client unhandled message %+v", m))
		}}
func formatMsg() *message {
	return &message{
		SrcHost:	clientID,
		SrcMod:		msgModClient,
		ConfHash:	confFileHash,
		Time:		time.Now(),
		Argc:		1,
		Argv:		make([]string,1),
		}}

func clusterStatus(delay int){
	var m *message
	waitForClientID()
	for{
		m=formatMsg()
		m.DestHost = "__master__"
		m.RpcFunc = clientAskAboutStatus
		coutgoing <- *m
		if delay == -1 {
			exitAfterTextReply = true
			return
		}else{
			time.Sleep(time.Duration(delay) * time.Second)}}}

func attachToClusterLogger(verbosity int){
	var m message
	waitForClientID()
	m = *formatMsg()
	m.DestHost = "__any__"
	m.RpcFunc = clientListenToClusterLogger
	m.Argv[0] = strconv.Itoa(verbosity)
	coutgoing <- m}

func (m *message)msg_handle_cluste_assigned_id() bool {
	// TODO better validation
	if m.RpcFunc == exchangeSendClientID {
		clientID = m.Argv[0]
		clogger(2, "Received client Id=" + clientID)
		clientIDSet = true
		return true}
	return false}

func (m *message)msg_handle_client_print() bool {
	if	m.ConfHash == confFileHash &&
		m.RpcFunc == clientPrintText &&
		m.SrcMod == msgModLoggr &&
		m.DestMod == msgModClient {
		
		fmt.Println(m.SrcHost, m.Time, "\t==> ", m.Argv[0])
		if exitAfterTextReply {
			os.Exit(0)}
		return true}
	return false}

var resValidStates []string = []string{
	"on",
	"off",
	"paused",
	"reboot",
	"nuke",
	}

func resourceMod(resName *string, resDesiredState *string) {
	for _,v := range resValidStates{
		if v == *resDesiredState { 
			goto success}}
	clogger(0, "-state received wrong argument")
	os.Exit(3) 
	
	success: 
	waitForClientID()
	coutgoing <- message{
		SrcHost:	clientID,
		DestHost:	"__any__",
		SrcMod:		msgModClient,
		DestMod:	msgModConfig,
		Argv:		[]string{
			*resName,
			*resDesiredState,}}
	clogger(3, "resourceMod message sent")
	os.Exit(0)
	}


func client(){
	// cli arguments
	var resName				string
	var resUUID				string
	var resDesiredState		string
	var clusterOp			string
	var socketPath			string
	
	var statusInterval		int
	var clusterLogLevel		int
	
	var statusFlag			bool
	var loggerFlag			bool
	var resModFlag			bool
	
	var unixConn			net.Conn
	var err					error
	
	cincoming = make(chan message)
	coutgoing = make(chan message)
	
	
	//var clientID		uint	= uint(rand.Uint32())
	
	flag.StringVar(&resName, "res", "VM_ubuntu_20.04",
		"name of resource you're trying to modify")
	
	flag.StringVar(&resUUID, "UUID",
		"8c56a50e-edf5-43f7-915e-5f34101bf4bf",
		"UUID of resource you're trying to modify")
	
	flag.StringVar(&resDesiredState, "state",
		"null", "state of resource you want to set")
	
	flag.StringVar(&clusterOp, "op", "maintenance",
		"cluster command you want to execute")
	
	flag.StringVar(&socketPath, "socket", "/run/lvcl.sock",
		"path to lvcl socket")
	
	flag.IntVar(&cliLogLevel, "v", 5,
		"verbosity of the client logger ")
	
	flag.IntVar(&clusterLogLevel, "cv", 1,
		"verbosity of the cluster logger ")
	
	flag.BoolVar(&statusFlag, "s", false,
		"display cluster status, (use -st to control delay or display once")

	flag.BoolVar(&loggerFlag, "w", false,
		"display cluster log")

	flag.BoolVar(&resModFlag, "r", false,
		"modify resource state")

	flag.IntVar(&statusInterval, "st", 1,
		"sleep between printing status (-1 displays status once) ")
	
	flag.Parse()
	
	fmt.Println(resName, resUUID, resDesiredState, 
		clusterOp, socketPath, cliLogLevel)
	
	clogger(1, "lvcl started in client mode")
	clogger(2,fmt.Sprintln("resName", resName))
	clogger(2,fmt.Sprintln("resUUID", resUUID))
	clogger(2,fmt.Sprintln("resDesiredState", resDesiredState))
	clogger(2,fmt.Sprintln("clusterOp", clusterOp))
	clogger(2,fmt.Sprintln("socketPath", socketPath))
	clogger(2,fmt.Sprintln("statusFlag", statusFlag))
	clogger(2,fmt.Sprintln("resModFlag", resModFlag))
	clogger(2,fmt.Sprintln("statusInterval", statusInterval))
	
	
	myHostname,err=os.Hostname()
	if(err != nil){
		panic(err)}
	file, err := os.Open("cluster.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(10)}
	var h=sha256.New()
	if _,err:=io.Copy(h,file);err!=nil {
		fmt.Println("ERR hashing cluster.json")}
	confFileHash = fmt.Sprintf("%x",h.Sum(nil))
	confFileHashRaw = h.Sum(nil)
	file.Close()
	clogger(2,"config hash: " + confFileHash)
	
	err=nil
	unixConn,err = net.Dial("unix", socketPath)
	if err!=nil {
		cerr("failed to connect to socket",err)
		os.Exit(2)}
	ec=eclient{
		//hostname:		fmt.Sprintf("lvcl-client-%d", clientID),
		hostname:		myHostname,
		originLocal:	true,
		outgoing:		coutgoing,
		incoming:		cincoming,
		conn:			unixConn,
		}
	go ec.forward()
	go ec.listen()
	
	if statusFlag && ! loggerFlag && ! resModFlag {
		go clusterStatus(statusInterval)}
	if loggerFlag && ! statusFlag && ! resModFlag {
		go attachToClusterLogger(clusterLogLevel)}
	if resModFlag && ! statusFlag && ! loggerFlag  {
		go resourceMod(&resName, &resDesiredState)}
		
	clientMessageHandler()
	time.Sleep(time.Millisecond * time.Duration(1000))
	}

