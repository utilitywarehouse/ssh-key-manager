# ssh-key-manager

[![Build Status](https://drone.prod.merit.uw.systems/api/badges/utilitywarehouse/ssh-key-manager/status.svg)](https://drone.prod.merit.uw.systems/utilitywarehouse/ssh-key-manager)

- allows users to set their ssh public keys in their Google GSuite account
- does a periodic sync of all specified groups (members + ssh keys) from
  Google to AWS s3

### server

Required environment variables:

| env var                   | example                         | desc                                                                           |
| -------                   | -------                         | ----                                                                           |
| SKM_CLIENT_ID             | xxx.apps.googleusercontent.com  | Google oidc client id                                                          |
| SKM_CLIENT_SECRET         | xxxxxxxx                        | Google oidc client secret                                                      |
| SKM_CALLBACK_URL          | https://app/callback            | Callback URI where user will be redirected after successful Google interaction |
| SKM_AWS_BUCKET            | bucket-name                     | AWS s3 bucket name                                                             |
| SKM_SA_KEY_LOC            | /etc/skm/sa-key.json            | Location on disk where Google service account key is (json format)             |
| SKM_GROUPS                | "group@gsuite-domain.com"       | comma seperated list of groups that will be synced to s3                       |
| SKM_ADMIN_EMAIL           | "admin-user@gsuite-domain.com"  | A G-Suite admin user                       |

You will also need to configure the appropriate AWS credentials for your
environment, as detailed [on this
page](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).

### client

Use https://github.com/utilitywarehouse/ssh-key-agent on your host to populate
`authorized_keys`
