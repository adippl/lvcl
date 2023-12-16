#!/bin/bash
#         number of lines   - numberof comment lines     - number of files * length of licence header
echo $(( $(cat *.go |wc -l) - $(grep -n '//' *.go|wc -l) - $(ls -1 *.go | wc -l) * 17 ))
