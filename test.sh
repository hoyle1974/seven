#!/bin/bash

c=0
while [ true ]
do
	u=`uuidgen`
	c=$(( $c + 1 ))
	curl -X POST localhost:8080/register -H "Content-Type: application/json" -d "{\"uuid\":\"$u\",\"addr\":\"$c\"}"
	echo
done
