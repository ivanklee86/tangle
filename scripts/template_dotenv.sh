#!/bin/sh

base_token=`argocd account generate-token --account automation --grpc-web`
prod_token=`argocd account generate-token --account automationProd --grpc-web`

echo "{\"base_token\": $base_token, \"prod_token\": $prod_token }" | gomplate -f .env.tmpl -d input=stdin:?type=application/json > .env
