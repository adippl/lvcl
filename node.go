package main

const(
	NodeOffline=iota
	NodePreparing
	NodeOnline
	NodeEvacuate
	)

type Node struct{
	Hostname	string
	TransportAddress	string
	NodeState int //{ NodeOffline, NodePreparing, NodeOnline, NodeEvacuate }
	Weight	int
	
	VCores	int
	HwCores	int
	
	VMem	int
	HwMem	int
	}



