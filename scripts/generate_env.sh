#!/bin/bash
# Environment hook for cloud-init image generation.
# Reads SSH public key and generates password hash, then outputs JSON with additional environment variables.

# Read input JSON from stdin
read -r input_json

# Read SSH public key
ssh_public_key=""
ssh_key_path="$HOME/.ssh/id_rsa.pub"
if [ -f "$ssh_key_path" ]; then
    ssh_public_key=$(cat "$ssh_key_path")
else
    echo "Warning: SSH public key not found at $ssh_key_path" >&2
fi

# Generate a simple password hash (in practice you'd want to use mkpasswd)
# For now, we'll use a simple hash that works for testing
password_hash='$6$rounds=4096$dZvpjkhL4EwsC3Wi$lJ8pB0hyROPYiWkCV0meWs9sqYTgiNnXxzBCn/XztnnwHBJVU11/0yRnsCrlpBKrH8k4xvlkVPbcPcqSt.tTL0'

# Generate instance ID (simple timestamp-based)
instance_id="cloudvm-$(date +%s)"

# Check if jq is available
if command -v jq >/dev/null 2>&1; then
    # Use jq to merge the input JSON with additional environment variables
    # This preserves all existing input values and adds/overwrites specific ones
    echo "$input_json" | jq -c --arg ssh_key "$ssh_public_key" \
                           --arg password_hash "$password_hash" \
                           --arg instance_id "$instance_id" \
                           '. + {
                             ssh_public_key: $ssh_key,
                             root_password_hash: $password_hash,
                             instance_id: $instance_id
                           }'
else
    # Fallback: simple JSON merging without jq
    # Parse existing values from input JSON
    hostname=$(echo "$input_json" | sed -n 's/.*"hostname":"\([^"]*\)".*/\1/p')
    if [ -z "$hostname" ]; then
        hostname="cloudvm"
    fi
    
    # Escape JSON strings properly
    ssh_public_key_escaped=$(printf '%s' "$ssh_public_key" | sed 's/\\/\\\\/g; s/"/\\"/g')
    password_hash_escaped=$(printf '%s' "$password_hash" | sed 's/\\/\\\\/g; s/"/\\"/g')
    instance_id_escaped=$(printf '%s' "$instance_id" | sed 's/\\/\\\\/g; s/"/\\"/g')
    hostname_escaped=$(printf '%s' "$hostname" | sed 's/\\/\\\\/g; s/"/\\"/g')
    
    # Output merged JSON (this is a simplified merge - only handles basic cases)
    printf '{"hostname":"%s","ssh_public_key":"%s","root_password_hash":"%s","instance_id":"%s"}\n' \
      "$hostname_escaped" \
      "$ssh_public_key_escaped" \
      "$password_hash_escaped" \
      "$instance_id_escaped"
fi 