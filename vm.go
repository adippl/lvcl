package main

import "os"
import "fmt"
import "io/ioutil"
import "encoding/json"

type VM struct{
	Name string
	DomainDefinition string
	VCpus int
	HwCpus int
	 // all memory counter in MiB
	VMem int
	HwMem int
	
	StorageClass int
	MigrationTimeout int
	MigrateLive bool
	}


func NewVM()*VM{
	return &VM{Name:"",
		DomainDefinition:"/dev/null",
		MigrateLive: true,
		MigrationTimeout: 180}}

func vmPrint(p *VM){
	fmt.Printf("%+v \n", *p)
	}

func VMReadFile(path string){
	fmt.Println("reading "+path)
	file, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(11)}
	defer file.Close()
	fmt.Println("reading "+path )	
	raw, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println("ERR reading "+path )
		os.Exit(11);}
	
	var vm VM
	json.Unmarshal(raw,&vm)
	//vmPrint(&vm)
	config.VMs = append(config.VMs,vm)
	fmt.Println("\n\n\n\nTESTESTSETSETSET\n\n\n")
	fmt.Println(config.VMs)
	
	confPrint(&config)
	
	}
