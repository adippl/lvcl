lvcl_PID := $(shell pgrep lvcl)
#ifeq ($(lvcl_PID),)
#	killCommand := $(kill -6 $(lvcl_PID))
#endif

CC=gccgo
CFLAGS=-Wall -Wextra -Wpedantic -march=skylake -static -g1
LDFLAGS= -static
SOURCES=$(wildcard *.go )
OBJECTS=$(SOURCES:.go=.o)
EXECUTABLE=lvcl

build:
	go build

push:
	git push home master
	git push github master



servers_boot:
	ipmitool -v -I lanplus -H r210II-1.idrac -U root -P $(pass homelab/r210II-1-6g) -y $(cat ~/.ssh/ipmikey) power on
	ipmitool -v -I lanplus -H r210II-2.idrac -U root -P $(pass homelab/r210II-2-fb) -y $(cat ~/.ssh/ipmikey) power on
	ipmitool -v -I lanplus -H r210II-3.idrac -U root -P $(pass homelab/r210II-3-91) -y $(cat ~/.ssh/ipmikey) power on

servers_shutdown:
	ipmitool -v -I lanplus -H r210II-1.idrac -U root -P $(pass homelab/r210II-1-6g) -y $(cat ~/.ssh/ipmikey) power soft
	ipmitool -v -I lanplus -H r210II-2.idrac -U root -P $(pass homelab/r210II-2-fb) -y $(cat ~/.ssh/ipmikey) power soft
	ipmitool -v -I lanplus -H r210II-3.idrac -U root -P $(pass homelab/r210II-3-91) -y $(cat ~/.ssh/ipmikey) power soft

wc:
	./wc.sh

client-test:
	for x in {1..10}; do ./lvcl-client -socket lvcl.sock -r -res 'basicResource 1' -state on; sleep 1 ; ./lvcl-client -socket lvcl.sock -r -res 'basicResource 1' -state off; sleep 1; done ; killall -s 6 lvcl

debugKill:
	#kill -6 $(lvcl_PID)
	killall -s 6 lvcl

$(EXECUTABLE): $(OBJECTS)
	$(CC) $(LDFLAGS) $(OBJECTS) -o $(EXECUTABLE)

$(OBJECTS): $(SOURCES)
	$(CC) $(CFLAGS) -c $< -o $@

clean:
	rm -f $(EXECUTABLE) $(OBJECTS)
