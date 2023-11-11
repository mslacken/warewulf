#!/usr/bin/bash
if [ -e /warewulf/warewulf.apikey ] ; then
        exit 0
fi
dd if=/dev/random bs=82 count=1 | base64 > /warewulf/warewulf.apikey
echo "Created API key"
cat /warewulf/warewulf.apikey
echo
tmpdir=$(mktemp -d)
cd $tmpdir
tpm2_startauthsession --session=session.ctx
tpm2_policypcr -Q --session=session.ctx --pcr-list="sha256:15" --policy=pcr15.sha256.policy
tpm2_createprimary -G ecc -Q --key-context=prim.ctx
(echo -n "WW_KEY:";cat /warewulf/warewulf.apikey) | tpm2_create -Q --policy=pcr15.sha256.policy --public=data.pub --private=data.priv -i - -C prim.ctx
tpm2_create -Q --policy=pcr15.sha256.policy --public=data_key.pub --private=data_key.priv -C prim.ctx
tpm2_load -Q --parent-context=prim.ctx --public=data.pub --private=data.priv --name=ww.apikey --key-context=data.ctx
tpm2_load -Q --parent-context=prim.ctx --public=data_key.pub --private=data_key.priv --name=wwkey --key-context=data_key.ctx
tpm2_evictcontrol -Q -c data.ctx
tpm2_evictcontrol -Q -c data_key.ctx
tpm2_flushcontext session.ctx
cd - > /dev/null
rm -rf $tmpdir
