#!/bin/sh

watcher(){
	while true ;do
		inotifywait *.go
		rm -rf lvcl.sock loc.log 
		clear;  go run .
		cat loc.log
		done
		}

watcher
