#!/bin/bash
. /lib/dracut-lib.sh

echo "This is Warewulf hybrid node installer"

test -e /etc/ww-hybrid.conf ||  die "Did not find warewulf configuration /etc/ww-hybrid.conf"

. /etc/ww-hybrid.conf


binsize_to_mbyte () {
    # """
    # converts binary sizes (1024k, 2.4G) to Megabytes
    # uses awk to handle floating point numbers
    # """
    local sz="$1"
    local bs=$(echo $sz | sed 's/\([0-9\.]*\)[GMBKkib]*/\1/')
    case ${sz} in
        *M|*MB|*Mb) mult=$((1)) ;;
	*MiB|*Mib) mult=$((1)) ;;
        *G|*GB|*Gb) mult=$((1024)) ;;
        *GiB|*Gib) mult=$((1000)) ;;
        *T|*Tb|*TB) mult=$((1024*1024)) ;;
        *Tb|*TB) mult=$((1000*1000)) ;;
        *) bs=${sz}; mult=1 ;;
    esac
    awk "BEGIN {print int(${bs}*${mult})}"
}

parse_yaml() {
    local yaml_file=$1
    local prefix=$2
    local s
    local w
    local fs

    s='[[:space:]]*'
    w='[a-zA-Z0-9_.-]*'
    fs="$(echo @ | tr @ '\034')"

    (
        sed -e '/- [^\“]'"[^\']"'.*: /s|\([ ]*\)- \([[:space:]]*\)|\1-\'$'\n''  \1\2|g' |
            sed -ne '/^--/s|--||g; s|\"|\\\"|g; s/[[:space:]]*$//g;' \
                -e 's/\$/\\\$/g' \
                -e "/#.*[\"\']/!s| #.*||g; /^#/s|#.*||g;" \
                -e "s|^\($s\)\($w\)$s:$s\"\(.*\)\"$s\$|\1$fs\2$fs\3|p" \
                -e "s|^\($s\)\($w\)${s}[:-]$s\(.*\)$s\$|\1$fs\2$fs\3|p" |
            awk -F"$fs" '{
            indent = length($1)/2;
            if (length($2) == 0) { conj[indent]="+";} else {conj[indent]="";}
            vname[indent] = $2;
            for (i in vname) {if (i > indent) {delete vname[i]}}
                if (length($3) > 0) {
                    vn=""; for (i=0; i<indent; i++) {vn=(vn)(vname[i])("_")}
                    printf("%s%s%s%s=(\"%s\")\n", "'"$prefix"'",vn, $2, conj[indent-1], $3);
                }
            }' |
            sed -e 's/_=/+=/g' |
            awk 'BEGIN {
                FS="=";
                OFS="="
            }
            /(-|\.).*=/ {
                gsub("-|\\.", "_", $1)
            }
            { print }'
    ) <"$yaml_file"
}

unset_variables() {
    # Pulls out the variable names and unsets them.
    #shellcheck disable=SC2048,SC2206 #Permit variables without quotes
    local variable_string=($*)
    unset variables
    variables=()
    for variable in "${variable_string[@]}"; do
        tmpvar=$(echo "$variable" | grep '=' | sed 's/=.*//' | sed 's/+.*//')
        variables+=("$tmpvar")
    done
    for variable in "${variables[@]}"; do
        if [ -n "$variable" ]; then
            unset "$variable"
        fi
    done
}

create_variables() {
    local yaml_file="$1"
    local prefix="$2"
    local yaml_string
    yaml_string="$(parse_yaml "$yaml_file" "$prefix")"
    unset_variables "${yaml_string}"
    eval "${yaml_string}"
}

create_parted_str() {
   parted "/dev/$1" print -m | \
	awk -F ":" '
	/'$1'/ {
		printf("'$1'_disk_size=\"%s\"\n",$2);
		printf("'$1'_label=\"%s\"\n",$6);
	}
	/^[1-4]/ {
		printf("'$1'_start+=(\"%s\")\n",$2);
		printf("'$1'_end+=(\"%s\")\n",$3);
		printf("'$1'_size+=(\"%s\")\n",$4);
		printf("'$1'_fstype+=(\"%s\")\n",$5);
		printf("'$1'_name+=(\"%s\")\n",$6);
	}
'
}

# get the wanted configuration
create_variables $DISK_CONFIG "conf_"
# get out the devices
devices=$(parse_yaml $DISK_CONFIG "conf_" |\
	grep conf_partitions_devices_ |\
	sed 's/conf_partitions_devices_\([[:alnum:]]*\)_partitions.*/\1/g' |\
	uniq )
if [ "x$devices" == "x" ] ; then
	echo "No configured devices"
	die
fi

# store if we have to format disk and how to this
format_disk=0
format_disk_str=()
mount_str=()
umount_str=()
fdisk_str=()
root_dev=""


for device in $devices; do
	test -e /dev/$device || die "Did not find desired device /dev/$device"
	# get actual disk configuration
	parted_str="$(create_parted_str $device)"
	eval "${parted_str}"

	declare -n real_disk_label=${device}_label
	# always recreate label when we format
	format_disk_str+=("parted /dev/$device mklabel ${conf_partitions_config_label} --script")
	if [ "x${conf_partitions_config_label}" != "x${real_disk_label}" ] ; then
		echo "label differs for /dev/${device}"
		format_disk=1
	fi
	declare -n conf_size_array="conf_partitions_devices_${device}_partitions__size"
	declare -n conf_type_array="conf_partitions_devices_${device}_partitions__type"
	declare -n conf_type_partnr="conf_partitions_devices_${device}_partitions__number"
	declare -n real_size_array="${device}_size"
	declare -n real_fs_array="${device}_fstype"
	declare -n real_name_array="${device}_name"
	part_start=0
	for i in {0..3} ; do 
		declare -n conf_fs="conf_filesystems_${device}${conf_type_partnr[$i]}_filesystem"
		echo $conf_fs | grep "ext" > /dev/null && create_opts="-F "
		echo $conf_fs | grep "btrfs" > /dev/null && create_opts="-F "
		[ -z $conf_fs ] && continue
		[ -z ${real_size_array[i]} ] && real_size="0" || real_size=${real_size_array[i]}
		real_size=$(binsize_to_mbyte $real_size)
		declare -n conf_mnt="conf_filesystems_${device}${conf_type_partnr[$i]}_mountpoint"
		declare -n conf_mnt_opts="conf_filesystems_${device}${conf_type_partnr[$i]}_mount_options"
		[ -z $conf_mnt_opts ] && conf_mnt_opts="defaults"
		if [ "x${conf_size_array[i]}" == "xrest" ] ; then
			#declare -n disk_size=${device}_disk_size
			#disk_size=$(binsize_to_mbyte $disk_size)
			#conf_size=$(($disk_size - $part_start - 250))
			format_disk_str+=("parted /dev/$device mkpart primary ${part_start} 100% --script")
		else	
			conf_size=$(binsize_to_mbyte ${conf_size_array[$i]})
			if [ "$real_size" != "$conf_size" ] ; then
				echo "Size differs for /dev/${device}$(($i + 1))"
				format_disk=1
			fi
			format_disk_str+=("parted /dev/$device mkpart primary ${part_start} $(($part_start + $conf_size)) --script")
		fi
		format_disk_str+=("parted /dev/$device name $(($i + 1)) WW$(echo $conf_mnt | sed 's/\//_/g') --script")
		
		format_disk_str+=("mkfs.${conf_fs} $create_opts /dev/${device}$(($i + 1))")
		part_start=$(($part_start + $conf_size))
		if [ "x${conf_mnt}" == "x/" ] ; then
			mount_str=("mount  -t ${conf_fs} -o $conf_mnt_opts /dev/${device}$(($i + 1)) ${NEWROOT}$conf_mnt" "${mount_str[@]}")
			umount_str=("${umount_str[@]}" "umount ${NEWROOT}$conf_mnt")
			fstab_str=("/dev/${device}$(($i + 1)) $conf_mnt $conf_fs $conf_mnt_opts 0 0" "${fstab_str[@]}")
			root_dev="/dev/${device}$(($i + 1))"
		else
			mount_str=("${mount_str[@]}" "mkdir -p ${NEWROOT}${conf_mnt}")
			mount_str=("${mount_str[@]}" "mount -t ${conf_fs} -o $conf_mnt_opts /dev/${device}$(($i + 1)) ${NEWROOT}$conf_mnt")
			umount_str=("umount ${NEWROOT}$conf_mnt" "${umount_str[@]}")
			fstab_str=("${fstab_str[@]}" "/dev/${device}$(($i + 1)) $conf_mnt $conf_fs $conf_mnt_opts 0 0")
		fi
		echo ${real_fs_array[$i]} | grep "fat" > /dev/null && real_fs="vfat" || real_fs=${real_fs_array[$i]}
		if [ "x${real_fs}" != "x$conf_fs" ] ; then
			echo "Filesystem differs on /dev/${device}$(($i + 1)) is ${real_fs}, wanted is $conf_fs"
			format_disk=1
		fi
		if [ "x${real_name_array[$i]}" != "xWW$(echo $conf_mnt | sed 's/\//_/g')" ] ; then
			echo "Partition name differs for /dev/${device}$(($i + 1))"
			format_disk=1
		fi

	done
done
if [ "x$format_disk" == "x1" ] ; then
	echo "ww-hyrid: wrong layout, formating disk now"
	for line in "${format_disk_str[@]}" ; do
		if [ "x$FORMAT" == "x1" ] ; then
			$line
			sleep 1
		fi

	done
else
	echo "ww-hyrid: layout is correct, mounting disk"
fi
for line in "${mount_str[@]}" ; do
	if [ "x$FORMAT" == "x1" ] ; then
		$line || die "Could not exec $line" && echo $line
	fi
done
# sync to the container
echo "ww-hybrid: running casync"
casync extract --reflink=no --with=unix $CASYNC_URL $NEWROOT/ || die "Could not run casync"
# bring down the nework
ifdown eth0
# sync the system overlay to the disk
tar cf - `cat $SYSTEM_OVERLAY_LIST` | tar -xf - -C $NEWROOT
# remove wrong stuff
rm -rf ${NEWROOT}/etc/sysconfig/network/ifcfg-*

# append the root device to the kernel 
#kexec --append=root=$root_dev
