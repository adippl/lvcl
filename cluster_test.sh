#!/bin/sh
set -v
set -e
make
scp lvcl lvcl-client h3:~/
scp lvcl lvcl-client h4:~/
scp lvcl lvcl-client h5:~/
urxvt -e ssh h3 'rm -f /root/{lvcl.sock,loc.log,cmb.log} && /root/lvcl' &
urxvt -e ssh h4 'rm -f /root/{lvcl.sock,loc.log,cmb.log} && /root/lvcl' &
urxvt -e ssh h5 'rm -f /root/{lvcl.sock,loc.log,cmb.log} && /root/lvcl' &

