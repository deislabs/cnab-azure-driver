#!/bin/bash
export TEST_CNAB_AZURE_SUBSCRIPTION_ID=
export TEST_CNAB_AZURE_TENANT_ID=
export TEST_CNAB_AZURE_CLIENT_ID=
export TEST_CNAB_AZURE_CLIENT_SECRET=
export TEST_CNAB_AZURE_LOCATION=
export TEST_CNAB_AZURE_STATE_FILESHARE=
export TEST_CNAB_AZURE_STATE_STORAGE_ACCOUNT_NAME=
export TEST_CNAB_AZURE_STATE_STORAGE_ACCOUNT_KEY=
export TEST_CNAB_AZURE_RUN_CLI_LOGIN_TEST=false
GO111MODULE=on go test -v -timeout 1h ./pkg/... -args -runazuretest -verbosedriveroutput