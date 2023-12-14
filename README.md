# hogeye
The HogEye CR was created to abstract away 99% of the needed yaml to deploy an app that will monitor a namespace in your cluster for old pods and notify you about them for management purposes. 

## Description
We've created a base app that will be deployed in a container running on a pod that is part of a deployment. This CR also creates a few other sub-resources to assist the app in it's goal of monitoring pods. It creates a service account, role, and role binding with the goal of giving the pod the ability to query the kube controller for the pod resources, along with the previously mentioned deployment.

We give the user the ability to make a yaml which only designates a few fields and accomplishes the goal above. The fields available to the user under spec:

- appTokenSecret <string> : name of the secret which will hold both tokens see "Creating Secrets" below

- slackChannels: <string> : Can be one of two options
    *  User id of a slack user to send notifications to. (Id is not the same as username. Can be found in user settings)
    *  Slack channel ID or NAME

- queryNamespace: <string> : The namespace for our app to be monitoring pods in

- queryTime: <string> : Cron job schedule 
    * If not familiar, Cron jobs use the following format
        * "minute hour year month day"
        * ex: "00 13 * * 1-5" <-- schedule is mon-fri at 1pm 
        * ex: "30 16 * 12 *" <-- schedule is Everyday in Dec at 4:30pm
        * ex: "45 8 * * *" <-- schedule is everyday of the year at 8:45am

- ageThreshold <int> : Integer representing the age of the pods in hours after which we'd want to get a notification.

## Example HogEye.yaml
```
apiVersion: hog.immortal.hedgehogs/v1
kind: HogEye
metadata:
  name: the-eye
spec:
  appTokenSecret: "hogeye-secrets"
  slackChannels: "U051DPPFNNT"
  queryNamespace: default
  queryTime: "42 14 * * *"
  ageThreshold: 0
```

## Caveats
Currently due to the use of a role and rolebinding we are only allowing to query for pods in the same namespace that the hogeye resource is deployed to. Be sure to make sure that the query namespace is set to the same namespace that the CR is deployed to.


## Getting Started

### Prerequisites
- go version v1.20.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### Notes on Deployment and Running
If you're running this using ```make run```, make sure to run ```export ENABLE_WEBHOOKS=false``` first, otherwise it will error out.

Make sure to install cert-manager to your cluster before running ```make deploy``` using ```kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.2/cert-manager.yaml```. Also make sure you're specifying an image in your make deploy, like so: ```make deploy IMG=<some-registry>/<project-name>:tag```

This CR deploys an app that was created by the #immortal Hedgehogs apprentice group at Liatrio. That being said there are app and bot tokens the app need to connect to slack. These are stored on the cluster as a secret that needs to be created beforehand. 

```sh
kubectl create secret opaque --from-file=appsecret=<file_containing_app_token.txt> --from-file=botsecret=<file_containing_bot_token.txt>
```

## To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**


`IMG=<your_registry>/hogeye:tag` as defined below should be set at the top of the make file! This registry will hold the image for your CR's manager. 

```sh
make docker-build docker-push IMG=<some-registry>/hogeye:tag
```

**NOTE:** This image ought to be published in the personal registry you specified. 
And it is required to have access to pull the image from the working environment. 
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/hogeye:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin 
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

