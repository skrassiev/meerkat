#!/bin/bash

if [ $# -ne 2 ]; then
	echo Usage: $0 --block=\(true\|false\) --fqdn=\<a.b.c.com\>
	exit
fi


for i in "$@"; do
	case $i in
		--block=*)
			BLOCK="${i#*=}"
			shift
			;;
		--fqdn=*)
			FQDN="${i#*=}"
			shift
			;;
		*)
			;;
	esac
done


if [[ -z $BLOCK || -z $FQDN ]]; then
	echo Usage: $0 --block=\(true\|false\) --fqdn=\<a.b.c.com\>
	exit
fi

if [ "$BLOCK" == "true" ]; then
	dig $FQDN +short|grep -v $FQDN | while read IP; do
		iptables -t filter -A OUTPUT -p tcp -d $IP -j DROP
	done
else
	dig $FQDN +short|grep -v $FQDN | while read IP; do
		iptables -t filter -L OUTPUT --line-numbers -n | grep $IP|sort -n -r|while read ln; do
			RULE=$(echo $ln | awk '{print $1}')
			iptables -t filter -D OUTPUT $RULE
		done
	done

fi
