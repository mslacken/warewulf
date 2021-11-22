#!/bin/bash

# called by dracut
check() {
    return 255
}

# called by dracut
depends() {
    echo network dm kernel-modules
    return 0
}

# called by dracut
installkernel() {
    #instmods =drivers/md dm_mod dm-cache dm-cache-mq dm-cache-cleane
    instmods =drivers/md dm_mod dm-cache dm-cache-mq dm-cache-cleaner
}

# called by dracut
install() {
    inst_multiple -o grep parted \
                     lsblk partprobe mdadm dcounter mkswap curl head md5sum resize2fs mkfs \
                     mkfs.ext2 mkfs.ext3 mkfs.ext4 mkfs.fat mkfs.vfat sync cryptsetup \
                     tail wipefs df casync /usr/lib/casync/protocols/casync-http chmod tar

     # install hook quite early, as we have to set the root argument	
     inst_hook cmdline 90 "$moddir/ww-parse-root.sh"
     inst_hook pre-mount 90 "$moddir/ww-hybrid.sh"
#    inst_hook initqueue/timeout 99 "$moddir/ww-hybrid-timeout.sh"

    echo "rd.neednet=1 rd.auto" > "${initdir}/etc/cmdline.d/50ww-hybrid.conf"

}

