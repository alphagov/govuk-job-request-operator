# GOV.UK Job Request Operator

This is a k8s operator that is used to make job requests built with [kube-builder](https://github.com/kubernetes-sigs/kubebuilder).

## Usage

To install the required dependencies:

```shell
brew install kubebuilder
brew install helm
brew install k3d
```

### Custom Resource Definition (CRD)

```
apiVersion: platform.gov.uk/v1
kind: JobRequest
metadata:
  labels:
    app.kubernetes.io/name: govuk-job-request-operator
    app.kubernetes.io/managed-by: kustomize
  name: jobrequest-sample
spec:
  foo: bar
```

### Create and generate the manifests

1. Create the manifests

```
make manifests
```

2. Start a k3d cluster

```
k3d cluster create cluster --api-port 6550
```

3. Install the CRDs into the cluster

```
make install
```

### Run the controller locally

1. Run the controller locally

This will run the controller locally and not in the cluster.

```
make run
```

2. Create a custom resource

Edit `platform_v1_jobrequest.yaml` to the following:

```
spec:
    foo: bar
```

3. Apply the resource

```
kubectl apply -k config/samples
```

### Run the controller in the cluster

1. Build the controller in a docker image

```
make docker-build
```

2. Modify the `manager` manifest

Edit the `manager` Deployment in `config/manager/manager.yaml` to include the following:

```
imagePullPolicy: IfNotPresent
```

3. Load the image into the cluster

```
k3d image import controller:latest -c cluster
```

### Generate Helm chart

1. Generate a Helm chart

```
kubebuilder edit --plugins=helm/v2-alpha
```

## Release a new version

This project uses [Semantic Versioning](https://semver.org/).

To create a new release, use the [Create Versioned Release](https://github.com/alphagov/govuk-job-request-operator/actions/workflows/release.yaml)
GitHub Actions workflow.
Select the correct version bump level (patch, minor or major) based on the changes made since the last release.

The release process works as follows:

1. 'Create Versioned Release' is triggered manually
2. A Git tag is calculated based on the provided version bump level and the latest version number
3. `goreleaser release --clean` runs, which:
   1. Builds the operator for macOS and Linux, arm64 and x86
   2. Generates CRD resources
   3. Packages up the binary and CRD resources into a .tar.gz
   4. Creates a GitHub Release with the packaged binary and creates a changelog based on commits since last release
   5. Builds a container image


## Team

[GOV.UK Platform
Engineering](https://github.com/orgs/alphagov/teams/gov-uk-platform-engineering)
team looks after this repo. If you're inside GDS, you can find us in
[#govuk-platform-engineering] or view our [kanban
board](https://github.com/orgs/alphagov/projects/71).

## Licence

[MIT License](LICENCE)

[#govuk-platform-engineering]: https://gds.slack.com/channels/govuk-platform-engineering
