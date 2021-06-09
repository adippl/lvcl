package main
import "testing"


var TestsGetVMbyName = []struct {
	input	string
	expectd	string
}{
	{"gh-test", "gh-test"},
	{"gh-test1", "gh-test1"},
	{"gh-test2", "gh-test2"},
	{"gh-test3", "gh-test3"},
	{"gh-test4", "gh-test4"},
	{"adawdawdwa", ""},
	{"≠²³≠≠≠gh-test4", ""},
	{"", ""},
	{"asdasdawda121302803912830123", ""},
	{"≠²³≠²³≠¢³¢€¢€§€·§½", ""},
}

func TestGetVMbyName(t *testing.T) {
	writeExampleConfig()
	confLoad()
	for _, tt := range TestsGetVMbyName {
		actual := config.GetVMbyName(&tt.input)
		if actual != nil && actual.Name != tt.expectd {
			t.Errorf("GetVMByDomain(%s): expected %v, actual %v", tt.input, tt.expectd, actual)
		}
	}
}

var TestsGetVMbyDomain = []struct {
	input	string
	expectd	string
}{
	{"tests struct embedded in main cluster.conf", "gh-test"},
	{"/mnt/cephvm/libvirt/domains/gh-test1.xml", "gh-test1"},
	{"/mnt/cephvm/libvirt/domains/gh-test2.xml", "gh-test2"},
	{"/mnt/cephvm/libvirt/domains/gh-test3.xml", "gh-test3"},
	{"/mnt/cephvm/libvirt/domains/gh-test4.xml", "gh-test4"},
	{"adawdawdwa", ""},
	{"≠²³≠≠≠gh-test4", ""},
	{"", ""},
	{"asdasdawda121302803912830123", ""},
	{"≠²³≠²³≠¢³¢€¢€§€·§½", ""},
}

func TestGetVMbyDomain(t *testing.T) {
	writeExampleConfig()
	confLoad()
	for _, tt := range TestsGetVMbyDomain {
		actual := config.GetVMbyDomain(&tt.input)
		if (actual != nil && actual.Name != tt.expectd) || (actual == nil && tt.expectd != "")  {
			t.Errorf("GetVMByDomain(%s): expected %v, actual %v", tt.input, tt.expectd, actual)
		}
	}
}


var TestsGetNodebyHostname = []struct {
	input	string
	expectd	string
}{
	{"r210II-1", "r210II-1"},
	{"r210II-2", "r210II-2"},
	{"r210II-3", "r210II-3"},
	{"notExisting node 1", ""},
	{"notExisting node 2", ""},
	{"notExisting node 3", ""},
	{"notExisting node 4", ""},
	{"notExisting node 5", ""},
}

func TestGetNodebyHostname(t *testing.T) {
	writeExampleConfig()
	confLoad()
	for _, tt := range TestsGetNodebyHostname {
		actual := config.GetNodebyHostname(&tt.input)
		if actual != nil && actual.Hostname != tt.expectd {
			t.Errorf("GetNodebyHostname(%s): expected %v, actual %v", tt.input, tt.expectd, actual)
		}
	}
}

var TestsCheckIfNodeExists = []struct {
	input	string
	expectd	bool
}{
	{"r210II-1", true},
	{"r210II-2", true},
	{"r320-1", true},
	{"notExisting node 1", false},
	{"notExisting node 2", false},
	{"notExisting node 3", false},
	{"notExisting node 4", false},
	{"notExisting node 5", false},
}

func TestCheckIfNodeExists(t *testing.T) {
	writeExampleConfig()
	confLoad()
	for _, tt := range TestsCheckIfNodeExists {
		actual := config.CheckIfNodeExists(&tt.input)
		if actual != tt.expectd {
			t.Errorf("CheckIfNodeExists(%s): expected %v, actual %v", tt.input, tt.expectd, actual)
		}
	}
}


//var TestsFUNC = []struct {
//	input	string
//	expectd	bool
//}{
//	{"r210II-1", true},
//	{"r210II-2", true},
//	{"r210II-3", true},
//	{"notExisting node 1", false},
//	{"notExisting node 2", false},
//	{"notExisting node 3", false},
//	{"notExisting node 4", false},
//	{"notExisting node 5", false},
//}
//
//func TestFUNC(t *testing.T) {
//	writeExampleConfig()
//	confLoad()
//	for _, tt := range TestsFUNC {
//		actual := config.FUNC(&tt.input)
//		if actual.Hostname != tt.expectd {
//			t.Errorf("FUNC(%s): expected %v, actual %v", tt.input, tt.expectd, actual)
//		}
//	}
//}
