#!/usr/bin/env sh

cmd="go run ./vendor/github.com/GoogleCloudPlatform/golang-samples/spanner/spanner_snippets/snippet.go"
db=projects/x/instances/y/databases/z
# As of writing, this one hits a non-existent endpoint. The protos are
# available at
# https://github.com/googleapis/googleapis/tree/4d41b11ed4f6378fca83cb6d77ceba732f9ac373/google/spanner/admin/database/v1,
# but for the time being, it seems reasonable to manually have to create a
# database.
# ${cmd} createdatabase $db

# As of writing, these hit an existing dummy endpoint and receive an
# "unimplemented" error.
${cmd} write $db
echo
${cmd} read $db
echo
