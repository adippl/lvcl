package main

import "fmt"
import "os"
import "encoding/json"
import "io/ioutil"

const( confDir="./" )
const( confFile="cluster.json" )

type Conf struct {
	UUID string
	DomainDefinitionDir string
	Nodes []Node
	VMs []VM
	Quorum int `json:"test"`
	BalanceMode int
	ResStickiness int
	GlobMigrationTimeout int
	GlobLiveMigrationBlock bool
	Maintenance bool
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
		
	json.Unmarshal(raw,&config)

	VMReadFile("domains/gh-test4.json") 

	loadAllVMfiles()

	fmt.Println("\n\n\n\n LOADED CONFIG\n\n\n\n")
	dumpConfig()

	}

func loadAllVMfiles(){
	f, e := ioutil.ReadDir("domains")
	if e!=nil{
		fmt.Println(e)}
	for _, f := range f{
		VMReadFile("domains/"+f.Name())}}

func dumpConfig(){
	raw, _ := json.MarshalIndent(&config,"","	")
	fmt.Println(string(raw))
	}

