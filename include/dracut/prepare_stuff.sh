#!/bin/bash
DRACUT_DIR=~/dracut
CONTAINER=leap-15
KERNEL=leap15-3-x86
CASYN_DIR=/srv/www/htdocs
rm -rf $DRACUT_DIR/*
dracut -dv --no-early-microcode \
	-m "base ww-hybrid"\
	--no-compress \
	--install 'chmod stat casync sleep /usr/lib/casync/protocols/casync-http ls find tr uniq' \
	--no-hostonly-cmdline \
	$DRACUT_DIR/test.initrd
mkdir $DRACUT_DIR/min-linux 
cd $DRACUT_DIR/min-linux 
cpio -id < $DRACUT_DIR/test.initrd
zcat /var/lib/warewulf/provision/kernel/${KERNEL}/kmods.img | cpio -id
mv init init.dracut 

find /var/lib/warewulf/overlays/system/default/ -type f | sed -e 's/\(.*\)\.ww$/\1/' -e 's,/var/lib/warewulf/overlays/system/default,,' > /var/lib/warewulf/overlays/system/default/etc/ww-system.lst
mkdir -p etc/
cat > etc/ww-hybrid.conf <<EOF
export CASYNC_URL="http://192.168.100.250/${CONTAINER}.caidx"
export SYSTEM_OVERLAY_LIST=/etc/ww-system.lst
export DISK_CONFIG=/etc/ww-disk.yaml
export WWROOTDEV=/dev/vda2
export FORMAT=1
EOF
cd -

cd $CASYNC_DIR
echo "Running casync"
casync make --with=unix --without=flag-nocow --without=flag-immutable --without=subvolume-ro --without=subvolume ${CONTAINER}.caidx /var/lib/warewulf/chroots/${CONTAINER}/rootfs/
cd -
wwctl container import $DRACUT_DIR/min-linux/ --force
wwctl overlay build -a
