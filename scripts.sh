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
	scp lvcl root@10.0.6.13:/root/
	}

sshHosts="root@10.0.6.11  root@10.0.6.12 root@10.0.6.13"
tmuxSetup(){
	tmux -L lvclTEST new-session -d ssh root@10.0.6.11 /root/lvcl 
#	tmux -L lvclTEST new-window -d ssh root@10.0.6.12 /root/lvcl
#	tmux -L lvclTEST new-window -d ssh root@10.0.6.13 /root/lvcl
	tmux -L lvclTEST split-window -d ssh root@10.0.6.12 /root/lvcl
	tmux -L lvclTEST split-window -d ssh root@10.0.6.13 /root/lvcl
	tmux -L lvclTEST select-layout even-vertical
	}
sshForAll(){
	for x in $sshHosts; do
		ssh $x "$@"
		done
		}

superWatcher(){
	while true ;do
		inotifywait *.go
		rm -rf lvcl.sock loc.log lvcl
		sshForAll rm /root/lvcl/
		clear;
		go build || continue
		push
		tmuxSetup
		sleep 1
		tmux -L lvclTEST attach
		ssh root@10.0.6.11 cat /root/loc.log
		done
		}

#watcher
superWatcher
