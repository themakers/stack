#!/usr/bin/env bash

set -e -u -x

USER_ID=$(id -u)
GROUP_ID=$(id -g)

export COMPOSE_MENU=false

function compose {
  docker compose \
    --file docker-compose.yml --project-name stack-demo \
    $@
}

case "$1" in
    "up")
        compose --env-file <(echo -e "USER_ID=$USER_ID\nGROUP_ID=$GROUP_ID") \
          up \
            --abort-on-container-exit --remove-orphans \
            --build --force-recreate --always-recreate-deps --timestamps
        ;;
    "down")
        compose down \
          --remove-orphans --volumes --rmi local
        ;;
    *)
        exit 64
        ;;
esac
