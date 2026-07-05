#!/bin/bash
# SSH wrapper with password

export SSHPASS="Zc2002119"
sshpass -e ssh -o StrictHostKeyChecking=no root@154.219.110.138 "$@"
