#!/bin/sh

watcher(){
	while true ;do
		inotifywait *.go
		rm -rf lvcl.sock loc.log 
		clear
		go run .
		cat loc.log
		done
		}

watcher_client(){
	while true ;do
		inotifywait *.go
		rm -rf lvcl.sock loc.log  2>&1 >/dev/null
		clear;
		go build
		./lvcl-client
#		cat loc.log
		done
		}

push(){
	scp lvcl root@10.0.6.14:/root/
	scp lvcl root@10.0.6.15:/root/
	scp lvcl root@10.0.6.16:/root/


	scp -r domains root@10.0.6.14:/root/
	scp -r domains root@10.0.6.15:/root/
	scp -r domains root@10.0.6.16:/root/
	}

sshHosts="root@10.0.6.14 root@10.0.6.15 root@10.0.6.16 "
tmuxSetup(){
	tmux -L lvclTEST new-session  -d ssh root@10.0.6.14 '/root/lvcl 2>&1 |tee lol' 
	tmux -L lvclTEST split-window -d ssh root@10.0.6.15 '/root/lvcl 2>&1 |tee lol'
	tmux -L lvclTEST split-window -d ssh root@10.0.6.16 '/root/lvcl 2>&1 |tee lol'
	#tmux -L lvclTEST select-layout even-horizontal
	tmux -L lvclTEST select-layout even-vertical
	}
sshForAll(){
	for x in $sshHosts; do
		ssh $x "$1"
		done
		}

superWatcher(){
	while true ;do
		inotifywait *.go
		rm -rf lvcl.sock loc.log lvcl
		sshForAll "rm /root/lvcl /root/loc.log /root/cmb.log /root/stdout "
		sshForAll "rm /root/lvcl.sock"
		sshForAll "killall lvcl"
		sleep 1
		clear;
		go build || continue
		push
		tmuxSetup
		tmux -L lvclTEST attach
		ssh root@10.0.6.14 cat /root/loc.log
		ssh root@10.0.6.14 cat /root/lol
		done
		}

#watcher_client
superWatcher
