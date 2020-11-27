package main
import "fmt"
import "encoding/json"
//import "os"
import "io/ioutil"
func bar(){
	fmt.Println("bar")
	test := Conf{ UUID:"testuuid", 
		DomainDefinitionDir: "domains/",
		Nodes: []Node{
			Node{
				Hostname: "r210II-1",
				TransportAddress: "10.0.6.11",
				NodeState: NodePreparing,
				Weight: 100},
			Node{
				Hostname: "r210II-2",
				TransportAddress: "10.0.6.12",
				NodeState: NodePreparing,
				Weight: 100},
			Node{
				Hostname: "r210II-3",
				TransportAddress: "10.0.6.13",
				NodeState: NodePreparing,
				Weight: 100}},
		VMs: []VM{
			VM{
				Name: "gh-test",
				DomainDefinition: "tests struct embedded in main cluster.conf",
				VCpus: 1,
				HwCpus: 0,
				VMem: 512,
				HwMem: 512,
				MigrationTimeout: 180,
				MigrateLive: true},
				},
		BalanceMode: Cpus,
		ResStickiness:50,
		GlobMigrationTimeout:120,
		GlobLiveMigrationBlock:false,
		Maintenance: true}
	
	fmt.Printf("%+v\n", test)
	//var x int = 5
	//confser, err := json.MarshalIndent(x,"","	")
	confser, err := json.MarshalIndent(test,"","	")
	//confser, err := json.Marshal(test)
	if err != nil {
		fmt.Println("Can't serislize", test)
		}
	fmt.Println(confser);
	
	ioutil.WriteFile("./cluster.json",confser,0644)
		

	}
