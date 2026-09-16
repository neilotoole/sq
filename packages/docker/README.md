# sq docker image

This is a docker image for `sq`. It is based on the `alpine` image and
includes a bunch of additional tools.

The image is published to both
[GitHub Container Registry](https://github.com/neilotoole/sq/pkgs/container/sq)
(`ghcr.io/neilotoole/sq`) and [Docker Hub](https://hub.docker.com/r/neilotoole/sq)
(`neilotoole/sq`). The two are the same multi-arch image (amd64 and arm64),
cosign-signed, carrying SLSA build provenance and an SPDX SBOM.

## Usage

### Docker

```shell
# Shell into a one-time container.
$ docker run -it ghcr.io/neilotoole/sq zsh

# Start container named "sq-shell" detached (in the background).
$ docker run -d --name sq-shell ghcr.io/neilotoole/sq
# Shell into that container.
$ docker exec -it sq-shell zsh
```

### Kubernetes

```shell
# Start pod named "sq-shell".
$ kubectl run sq-shell --image ghcr.io/neilotoole/sq
# Shell into the pod.
$ kubectl exec -it sq-shell -- zsh
```
