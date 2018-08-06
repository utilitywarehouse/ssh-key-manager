# ssh-key-manager

[![Docker Repository on Quay](https://quay.io/repository/utilitywarehouse/ssh-key-manager/status "Docker Repository on Quay")](https://quay.io/repository/utilitywarehouse/ssh-key-manager)

 - allows users to set their ssh public keys in their Google GSuite account
 - does a periodic sync of all specified groups (members + ssh keys) from Google to AWS s3

### server

Required environment variables:

| env var                   | example                         | desc                                                                           |
| -------                   | -------                         | ----                                                                           |
| SKM_CLIENT_ID             | xxx.apps.googleusercontent.com  | Google oidc client id                                                          |
| SKM_CLIENT_SECRET         | xxxxxxxx                        | Google oidc client secret                                                      |
| SKM_CALLBACK_URL          | https://app/callback            | Callback URI where user will be redirected after successful Google interaction |
| SKM_AWS_ACCESS_KEY_ID     | AKIAXXXXXXXXXXXXXXXX            | AWS access key                                                                 |
| SKM_AWS_SECRET_ACCESS_KEY | xxxxxxxxxxxxxxxxxxxxx           | AWS secret access key                                                          |
| SKM_AWS_BUCKET            | bucket-name                     | AWS s3 bucket name                                                             |
| SKM_SA_KEY_LOC            | /etc/skm/sa-key.json            | Location on disk where Google service account key is (json format)             |
| SKM_GROUPS                | "group@gsuite-domain.com"       | comma seperated list of groups that will be synced to s3                       |

### client

Use https://github.com/utilitywarehouse/ssh-key-agent on your host to populate `authorized_keys`
