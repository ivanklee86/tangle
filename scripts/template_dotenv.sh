#!/bin/sh

base_token=`argocd account generate-token --account automation --grpc-web`
test_token=`argocd account generate-token --account automationTest --grpc-web`

echo "{\"base_token\": $base_token, \"test_token\": $test_token }" | gomplate -f .env.tmpl -d input=stdin:?type=application/json > .env
