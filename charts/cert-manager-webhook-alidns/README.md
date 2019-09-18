# ACME webhook for alidns


## Installation

```bash
$ helm install --name cert-manager-webhook-alidns ./deploy/webhook-alidns
```

## Issuer

secret

```bash
$ kubectl -n cert-manager create secret generic alidns-credentials --from-literal=accessKeySecret='your alidns accesskeySecret'
```

RBAC

```yaml
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  name: cert-manager-webhook-alidns:secret-reader
rules:
  - apiGroups:
      - ''
    resources:
      - 'secrets'
    resourceNames:
      - 'alidns-credentials'
    verbs:
      - 'get'
      - 'watch'
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: cert-manager-webhook-alidns:secret-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cert-manager-webhook-alidns:secret-reader
subjects:
  - apiGroup: ""
    kind: ServiceAccount
    name: cert-manager-webhook-alidns
    namespace: default
```

ClusterIssuer

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: <your email>
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - selector: 
        dnsNames:
        - '*.example.cn'
      dns01:
        webhook:
          config:
            accessKeyId: <your alidns accessKeyId>
            accessKeySecretRef:
              key: accessKeySecret
              name: alidns-credentials
            regionId: "cn-beijing"
            ttl: 600
          groupName: acme.lin07.me
          solverName: alidns
```

Certificate

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: wildcard-example-cn
spec:
  secretName: wildcard-example-cn-tls
  renewBefore: 240h
  dnsNames:
  - '*.example.cn'
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
```

Ingress

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: example-ingress
  namespace: default
  annotations:
    certmanager.k8s.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - '*.example.cn'
    secretName: wildcard-example-cn-tls
  rules:
  - host: demo.example.cn
    http:
      paths:
      - path: /
        backend:
          serviceName: backend-service
          servicePort: 80
```

## Development

### Running the test suite



All DNS providers **must** run the DNS01 provider conformance testing suite,
else they will have undetermined behaviour when used with cert-manager.

**It is essential that you configure and run the test suite when creating a
DNS01 webhook.**

An example Go test file has been provided in [main_test.go]().

> Prepare

```bash
$ scripts/fetch-test-binaries.sh
```

You can run the test suite with:

```bash
$ TEST_ZONE_NAME=example.com go test .
```

The example file has a number of areas you must fill in and replace with your
own options in order for tests to pass.
