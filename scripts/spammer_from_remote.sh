#!/bin/bash

# This script is used to spam the edge with 50kb files
while :
do
  ms=$(date +%s%N)
  dd if=/dev/random of=random_"$ms".dat bs=10000 count=50
	curl --location --request POST 'http://localhost:1313/api/v1/content/add' \
  --header 'Authorization: Bearer EST9ca3c377-d422-4d85-adb4-4289f1f760e8ARY' \
  --form 'data=@"./random_'${ms}'.dat"' \
  --form 'miner="t017840"'
  rm random_$ms.dat
done