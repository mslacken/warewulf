#!/usr/bin/bash
for hdl in $(tpm2_getcap handles-persistent|awk '{print $2}'); do
    tpm2_startauthsession --policy-session --session=session.ctx
    tpm2_policypcr -Q --session=session.ctx --pcr-list="sha256:15" --policy=pcr15.sha256.policy
    unsealeddata=$(tpm2_unseal --auth=session:session.ctx -Q -c $hdl 2>/dev/null)
    publickey=$(tpm2_readpublic -c $hdl 2>/dev/null)
    attr=$(echo "$publickey" | yq ".attributes.value")
    echo "$hdl public $attr" 
    tpm2_flushcontext session.ctx
    if [[ $unsealeddata == "WW_KEY:"* ]]; then
        confluent_apikey=${unsealeddata#WW_KEY:}
        echo $confluent_apikey > /warewulf/warewulf.apikey
        echo "$hdl API: $confluent_apikey"
        if [ -n "$lasthdl" ]; then
            tpm2_evictcontrol -c $lasthdl
        fi
        lasthdl=$hdl
    fi
done
