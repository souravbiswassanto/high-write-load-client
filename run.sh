#! /usr/bin/bash

export $(cat .env | grep -v '^#' | xargs) && export TEST_RUN_DURATION=300 && export CONCURRENT_WRITERS=100 && export READ_PERCENT=60 && export INSERT_PERCENT=25 && export UPDATE_PERCENT=15 && export READ_BATCH_SIZE=20 && go run main_v2.go
