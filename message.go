package main

import "fmt"
//import "json"

const(
	mgsModUnd=iota
	mgsModLog
	msgModExc
	msgModBrn
	)

type message struct{
	SrcHost		string
	DestHost	string
	SrcMod		uint
	DestMod		uint
	RpcFunc		uint
	arg1		string
	}



func (p *message)dump(){
	fmt.Printf("%+v \n", *p)
	}
