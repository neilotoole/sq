# sq docker image

This is a docker image for `sq`. It is based on the `alpine` image and
includes a bunch of additional tools.

## Usage

### Docker

```shell
# Shell into a one-time container.
$ docker run -it ghcr.io/neilotoole/sq zsh

# Start container named "sq-shell" as a daemon.
$ docker run -d --name sq-shell ghcr.io/neilotoole/sq
# Shell into that container.
$ docker exec -it sq-shell zsh 
```

### Kubernetes

```shell
# Start pod named "sq-shell".
$ kubectl run sq-shell --image ghcr.io/neilotoole/sq
# Shell into the pod.
$ k exec -it sq-shell -- zsh 
```
