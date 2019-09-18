#!/usr/bin/env bash

IMAGE_NAME="lin07/cert-manager-webhook-alidns"
IMAGE_TAG="latest"

OUT=`pwd`/deploy

helm template \
        --name cert-manager-webhook-alidns \
        --set image.repository=$IMAGE_NAME \
        --set image.tag=$IMAGE_TAG \
        deploy/webhook-alidns > "$OUT/rendered-manifest.yaml"