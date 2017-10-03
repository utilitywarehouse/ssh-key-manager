# ssh-key-manager

 - allows users to set their ssh public keys against a custom field against their account in Google GSuite
 - does a periodic sync of all specified groups to get their members + their keys and serialise to a file in s3

### server

Required environment variables:

| env var                   | example                                                                  | desc                                                                           |
| -------                   | -------                                                                  | ----                                                                           |
| SKM_CLIENT_ID             | xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.apps.googleusercontent.com | Google oidc client id                                                          |
| SKM_CLIENT_SECRET         | xxxxxxxxxxxxxxxxxxxxxxxx                                                 | Google oidc client secret                                                      |
| SKM_CALLBACK_URL          | https://app/callback                                                     | Callback URI where user will be redirected after successful Google interaction |
| SKM_AWS_ACCESS_KEY_ID     | AKIAXXXXXXXXXXXXXXXX                                                     | AWS access key                                                                 |
| SKM_AWS_SECRET_ACCESS_KEY | xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx                                 | AWS secret access key                                                          |
| SKM_AWS_BUCKET            | bucket-name                                                              | AWS s3 bucket name                                                             |
| SKM_SA_KEY_LOC            | /etc/skm/sa-key.json                                                     | Location on disk where Google service account key is (json format)             |
| SKM_GROUPS                | "ssh-kube@utilitywarehouse.co.uk"                                        | comma seperated list of groups that will be synced to s3                       |

### client

Needs `curl` and `jq`, extract ssh keys, for a group:

```
curl -s https://[app|bucket]/authmap | jq -r '.[] | select (.name == "group@gsuite-domain.com") | .keys[]'
```
