#!/bin/bash

echo "==== Docker Container Status ===="
docker ps -a

echo ""
echo "==== Speicherplatz Nutzung ===="
df -h

echo ""
echo "==== RAM Nutzung ===="
free -h
