package main

import "os"
import "fmt"
import "io/ioutil"
import "encoding/json"

type VM struct{
	Name string
	DomainDefinition string
	VCpus uint
	HwCpus uint
	 // all memory counter in MiB
	VMem uint
	HwMem uint
	
	StorageClass int
	MigrationTimeout int
	MigrateLive bool
	}


func NewVM()*VM{
	return &VM{Name:"",
		DomainDefinition:"/dev/null",
		MigrateLive: true,
		MigrationTimeout: 180}}

func (p *VM)dump(){
	fmt.Printf("\n dumping VM %+v \nEND\n", *p)}

func VMReadFile(path string)(err error){
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

	_,err=config.getVMbyName(&vm.Name)
	if(err == nil){
		fmt.Println("err VM with name %s already exists",vm.Name)
		return fmt.Errorf("err VM with name %s already exists",vm.Name)}

	_,err=config.getVMbyDomain(&vm.DomainDefinition)
	if(err == nil){
		fmt.Println("DEBUG err VM with domain file %s already exists\n",vm.DomainDefinition)
		return fmt.Errorf("err VM with domain file %s already exists",vm.DomainDefinition)}
	
	//TODO temporaly disabled, checks if domain.xml file exists on filesystem
	//if(vm.validate()==false){
	//	fmt.Println("DEBUG err VM failed .validate")
	//	return fmt.Errorf("err VM failed .validate")}
	
	config.VMs = append(config.VMs,vm)
	fmt.Println("\n\n\n\nTESTESTSETSETSET\n\n\n")
	fmt.Println(config.VMs)
	
	confPrint(&config)
	
	return nil}

func (v *VM)validate()(passed bool){
	
	_,err:=os.Stat(v.DomainDefinition)
	if os.IsNotExist(err){
		fmt.Println("DEBUG err vm DomainDefinition xml file doesn't exists\n")
		return false}
	
	if(v.VCpus>=256){
		fmt.Println("WARN, High number of cores in vm")}
	return true
	}
