# ktransform

Kubernetes CRD and controller to transform Secrets and ConfigMaps using [jq](https://stedolan.github.io/jq/) queries.  

## Installation

Install CRDs:
```
kubectl apply -k github.com/mgoltzsche/ktransform/deploy/crds
```

Install the operator in the current namespace:
```
kubectl apply -k github.com/mgoltzsche/ktransform/deploy
```

## Usage

The following example transforms two docker registry Secrets and a ConfigMap into a [makisu config](https://github.com/uber/makisu#configuring-docker-registry) Secret.  

Create the input Secrets:
```
for i in 1 2; do
  kubectl create secret docker-registry regcred$i \
    --docker-server=registry${i}.example.org \
    --docker-username=usr --docker-password=pw$i \
    --docker-email=johndoe@example.org
done
```
Create an input ConfigMap:
```
kubectl create configmap myconf \
  --from-literal=myconf=$'registries:\n- registry0.example.org\n- registry1.example.org' \
  --from-literal=myval=somevalue
```
Merge and convert all three resources to a single Secret:
```
kubectl apply -f - <<-EOF
apiVersion: ktransform.mgoltzsche.github.com/v1alpha1
kind: SecretTransform
metadata:
  name: dockertomakisuconf
spec:
  input:
    secret1:
      secret: regcred1
    secret2:
      secret: regcred2
    config:
      configMap: myconf
  output:
  - secret:
      name: makisu-conf
      type: Opaque
    transformation:
      primary: .config.myconf.object.registries[0]
      secondary: .config.myconf.object.registries[1]
      myval: .config.myval.string
      makisu.conf: |
        (.secret1[".dockerconfigjson"].object.auths * .secret2[".dockerconfigjson"].object.auths) |
          with_entries(.value |= {
            ".*": {
              security: {
                basic: .auth | @base64d | split(":") | {
                  username: .[0],
                  password: .[1]
                }
              }
            }
          })
EOF
```

A `SecretTransform`'s status is reflected in its `Synced` condition.
In case of an error this condition provides more information.  

When the condition is met the Secret `makisu-conf` has been written:
```
$ kubectl get secret makisu-conf -o jsonpath='{.data.primary}' | base64 -d && echo
registry0.example.org
$ kubectl get secret makisu-conf -o jsonpath='{.data.secondary}' | base64 -d && echo
registry1.example.org
$ kubectl get secret makisu-conf -o jsonpath='{.data.myval}' | base64 -d && echo
somevalue
$ kubectl get secret makisu-conf -o jsonpath='{.data.makisu\.conf}' | base64 -d | jq .
{
  "registry1.example.org": {
    ".*": {
      "security": {
        "basic": {
          "password": "pw1",
          "username": "usr"
        }
      }
    }
  },
  "registry2.example.org": {
    ".*": {
      "security": {
        "basic": {
          "password": "pw2",
          "username": "usr"
        }
      }
    }
  }
}
```

When any input or output resource changes the transformation is reconciled.
If an input resource does not (yet) exist or is deleted the transformation is reconciled after 30 seconds.

## Updating workloads referring to transformation outputs

While ktransform continuously applies transformations when any input or output changes
it does **not** update Deployments/StatefulSets/DaemonSets that refer to output resources.
However this can be achieved using [wave](https://github.com/pusher/wave).

## How to build
```
make
```

## How to test

Run unit tests:
```
make unit-tests
```

Run e2e tests:
```
make start-minikube
make e2e-tests
```
