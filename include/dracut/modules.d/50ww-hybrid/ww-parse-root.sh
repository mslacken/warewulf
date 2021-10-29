#!/bin/bash
test -e /etc/ww-hybrid.conf ||  die "Did not find warewulf configuration /etc/ww-hybrid.conf"

. /etc/ww-hybrid.conf

export root=$WWROOTDEV
# Yes, root is not OK yet, but dracut wants is this way
export rootok=1
