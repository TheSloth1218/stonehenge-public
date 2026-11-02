#!/bin/sh
set -eu

/usr/sbin/nginx -t
/usr/sbin/nginx -s reload
