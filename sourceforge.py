# https://sourceforge.net/p/forge/documentation/Release%20Files%20for%20Download/#scp
import hashlib
import io
import os
import pathlib

from paramiko import SSHClient, client
from scp import SCPClient

ssh = SSHClient()
ssh.set_missing_host_key_policy(client.AutoAddPolicy())
ssh.connect(
    'frs.sourceforge.net',
    username=os.environ['SF_USER'],
    password=os.environ['SF_PASS']
)

scp = SCPClient(ssh.get_transport())

raw = open('bin/gw3', 'rb').read()
hex_ = hashlib.md5(raw).hexdigest()
print(hex_)

f = io.BytesIO(raw)
f.seek(0)

scp.putfo(f, '/home/frs/project/mgl03/gw3/' + hex_)
scp.close()
