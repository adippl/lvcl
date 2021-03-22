#!/bin/sh

watcher(){
	while true ;do
		inotifywait *.go
		rm -rf lvcl.sock loc.log 
		clear;  go run .
		cat loc.log
		done
		}

push(){
	scp lvcl root@10.0.6.11:/root/
	scp lvcl root@10.0.6.12:/root/
	scp lvcl root@10.0.6.14:/root/
	}

sshHosts="root@10.0.6.11 root@10.0.6.12 root@10.0.6.14"
tmuxSetup(){
	tmux -L lvclTEST new-session -d ssh root@10.0.6.11 '/root/lvcl |tee lol' 
	tmux -L lvclTEST split-window -d ssh root@10.0.6.12 /root/lvcl
	tmux -L lvclTEST split-window -d ssh root@10.0.6.14 /root/lvcl
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
		sshForAll "killall lvcl"
		sleep 1
		clear;
		go build || continue
		push
		tmuxSetup
		tmux -L lvclTEST attach
		ssh root@10.0.6.11 cat /root/loc.log
		ssh root@10.0.6.11 cat /root/lol
		done
		}

#watcher
superWatcher
