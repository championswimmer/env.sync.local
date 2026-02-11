#!/usr/bin/env bats

load 'test_helper'

@test "Clear existing secrets and initialize alpha with encryption" {
    run init_container "$CONTAINER_ALPHA" true
    [ "$status" -eq 0 ]
}

@test "Add encrypted secret to alpha" {
    run add_secret "$CONTAINER_ALPHA" "ENCRYPTED_SECRET" "secret-value-789"
    [ "$status" -eq 0 ]
}

@test "Verify encrypted file exists on alpha" {
    run parallel_run \
        "container_exec \"$CONTAINER_ALPHA\" test -f /home/envsync/.secrets.env" \
        "container_exec \"$CONTAINER_ALPHA\" grep -q \"ENCRYPTED: true\" /home/envsync/.secrets.env"
    [ "$status" -eq 0 ]
}

@test "Get alpha's AGE public key" {
    ALPHA_PUBKEY=$(get_pubkey "$CONTAINER_ALPHA")
    [ -n "$ALPHA_PUBKEY" ]
    echo "Alpha pubkey: $ALPHA_PUBKEY" >&3
    export ALPHA_PUBKEY
}

@test "Initialize beta with encryption" {
    run init_container "$CONTAINER_BETA" true
    [ "$status" -eq 0 ]
}

@test "Add beta-only encrypted secret before any key exchange" {
    run add_secret "$CONTAINER_BETA" "BETA_AUTO" "beta-auto-value"
    [ "$status" -eq 0 ]
}

@test "Beta syncs without manual key exchange (auto registration with alpha)" {
    run trigger_sync "$CONTAINER_BETA"
    [ "$status" -eq 0 ]
}

@test "Beta can decrypt alpha's encrypted secret after auto registration" {
    run get_secret "$CONTAINER_BETA" "ENCRYPTED_SECRET"
    [ "$status" -eq 0 ]
    [ "$output" = "secret-value-789" ]
}

@test "Alpha can decrypt beta's secret after alpha sync post auto-registration" {
    run trigger_sync "$CONTAINER_ALPHA"
    [ "$status" -eq 0 ]

    run get_secret "$CONTAINER_ALPHA" "BETA_AUTO"
    [ "$status" -eq 0 ]
    [ "$output" = "beta-auto-value" ]
}

@test "Initialize gamma with encryption" {
    run init_container "$CONTAINER_GAMMA" true
    [ "$status" -eq 0 ]
}

@test "Get beta's and gamma's AGE public keys" {
    local beta_pubkey_file
    local gamma_pubkey_file

    beta_pubkey_file=$(mktemp)
    gamma_pubkey_file=$(mktemp)

    run parallel_run \
        "get_pubkey \"$CONTAINER_BETA\" > \"$beta_pubkey_file\"" \
        "get_pubkey \"$CONTAINER_GAMMA\" > \"$gamma_pubkey_file\""
    [ "$status" -eq 0 ]

    BETA_PUBKEY=$(<"$beta_pubkey_file")
    [ -n "$BETA_PUBKEY" ]
    export BETA_PUBKEY
    echo "Beta pubkey: $BETA_PUBKEY" >&3

    GAMMA_PUBKEY=$(<"$gamma_pubkey_file")
    [ -n "$GAMMA_PUBKEY" ]
    export GAMMA_PUBKEY
    echo "Gamma pubkey: $GAMMA_PUBKEY" >&3

    rm -f "$beta_pubkey_file" "$gamma_pubkey_file"
}

@test "Exchange public keys between all containers" {
    local alpha_pubkey_file
    local beta_pubkey_file
    local gamma_pubkey_file
    local alpha_pubkey
    local beta_pubkey
    local gamma_pubkey

    alpha_pubkey_file=$(mktemp)
    beta_pubkey_file=$(mktemp)
    gamma_pubkey_file=$(mktemp)

    run parallel_run \
        "get_pubkey \"$CONTAINER_ALPHA\" > \"$alpha_pubkey_file\"" \
        "get_pubkey \"$CONTAINER_BETA\" > \"$beta_pubkey_file\"" \
        "get_pubkey \"$CONTAINER_GAMMA\" > \"$gamma_pubkey_file\""
    [ "$status" -eq 0 ]

    alpha_pubkey=$(<"$alpha_pubkey_file")
    beta_pubkey=$(<"$beta_pubkey_file")
    gamma_pubkey=$(<"$gamma_pubkey_file")

    rm -f "$alpha_pubkey_file" "$beta_pubkey_file" "$gamma_pubkey_file"

    [ -n "$alpha_pubkey" ]
    [ -n "$beta_pubkey" ]
    [ -n "$gamma_pubkey" ]

    run parallel_run \
        "import_pubkey \"$CONTAINER_ALPHA\" \"$beta_pubkey\" \"beta.local\" && import_pubkey \"$CONTAINER_ALPHA\" \"$gamma_pubkey\" \"gamma.local\"" \
        "import_pubkey \"$CONTAINER_BETA\" \"$alpha_pubkey\" \"alpha.local\" && import_pubkey \"$CONTAINER_BETA\" \"$gamma_pubkey\" \"gamma.local\"" \
        "import_pubkey \"$CONTAINER_GAMMA\" \"$alpha_pubkey\" \"alpha.local\" && import_pubkey \"$CONTAINER_GAMMA\" \"$beta_pubkey\" \"beta.local\""
    [ "$status" -eq 0 ]
}

@test "Trigger sync on all containers to exchange encrypted secrets" {
    run parallel_run \
        "trigger_sync \"$CONTAINER_ALPHA\"" \
        "trigger_sync \"$CONTAINER_BETA\"" \
        "trigger_sync \"$CONTAINER_GAMMA\""
    [ "$status" -eq 0 ]
}

@test "Verify beta can decrypt the secret" {
    run get_secret "$CONTAINER_BETA" "ENCRYPTED_SECRET"
    [ "$status" -eq 0 ]
    [ "$output" = "secret-value-789" ]
}

@test "Verify gamma can decrypt the secret" {
    run get_secret "$CONTAINER_GAMMA" "ENCRYPTED_SECRET"
    [ "$status" -eq 0 ]
    [ "$output" = "secret-value-789" ]
}

@test "Verify all containers have encrypted files" {
    run parallel_run \
        "container_exec \"$CONTAINER_ALPHA\" grep -q \"ENCRYPTED: true\" /home/envsync/.secrets.env" \
        "container_exec \"$CONTAINER_BETA\" grep -q \"ENCRYPTED: true\" /home/envsync/.secrets.env" \
        "container_exec \"$CONTAINER_GAMMA\" grep -q \"ENCRYPTED: true\" /home/envsync/.secrets.env"
    [ "$status" -eq 0 ]
}
