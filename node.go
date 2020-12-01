package main

import "fmt"

const(
	NodeOffline=iota
	NodePreparing
	NodeOnline
	NodeEvacuate
	)

type Node struct{
	Nodename	string
	LibvirtAddress	string
	NodeState int //{ NodeOffline, NodePreparing, NodeOnline, NodeEvacuate }
	Weight	int
	
	VCores	int
	HwCores	int
	
	VMem	int
	HwMem	int
	}



func (p *Node)dump(){
	fmt.Printf("\n dumping Node %+v \nEND\n", *p)}
